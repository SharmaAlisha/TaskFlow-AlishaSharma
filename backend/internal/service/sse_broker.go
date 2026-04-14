package service

import (
	"log/slog"
	"sync"

	"github.com/taskflow/backend/internal/model"
)

type SSEClient struct {
	ID       string
	UserID   string
	Events   chan model.Event
	Done     chan struct{}
}

type SSEBroker struct {
	mu       sync.RWMutex
	clients  map[string]map[string]*SSEClient // projectID -> clientID -> client
	eventCh  chan model.Event
	stop     chan struct{}
	logger   *slog.Logger
}

func NewSSEBroker(logger *slog.Logger) *SSEBroker {
	b := &SSEBroker{
		clients: make(map[string]map[string]*SSEClient),
		eventCh: make(chan model.Event, 256),
		stop:    make(chan struct{}),
		logger:  logger,
	}
	go b.run()
	return b
}

func (b *SSEBroker) run() {
	for {
		select {
		case evt := <-b.eventCh:
			b.fanOut(evt)
		case <-b.stop:
			return
		}
	}
}

func (b *SSEBroker) fanOut(evt model.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	clients, ok := b.clients[evt.ProjectID]
	if !ok {
		return
	}

	for _, client := range clients {
		select {
		case client.Events <- evt:
		default:
			b.logger.Warn("dropping SSE event for slow client",
				"client_id", client.ID,
				"project_id", evt.ProjectID,
			)
		}
	}
}

func (b *SSEBroker) Publish(evt model.Event) {
	select {
	case b.eventCh <- evt:
	default:
		b.logger.Warn("SSE event channel full, dropping event",
			"event_type", evt.Type,
			"project_id", evt.ProjectID,
		)
	}
}

func (b *SSEBroker) Subscribe(client *SSEClient, projectIDs []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, pid := range projectIDs {
		if b.clients[pid] == nil {
			b.clients[pid] = make(map[string]*SSEClient)
		}
		b.clients[pid][client.ID] = client
	}
}

func (b *SSEBroker) Unsubscribe(clientID string, projectIDs []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, pid := range projectIDs {
		if clients, ok := b.clients[pid]; ok {
			delete(clients, clientID)
			if len(clients) == 0 {
				delete(b.clients, pid)
			}
		}
	}
}

func (b *SSEBroker) Shutdown() {
	close(b.stop)

	b.mu.Lock()
	defer b.mu.Unlock()

	for pid, clients := range b.clients {
		for _, client := range clients {
			close(client.Done)
		}
		delete(b.clients, pid)
	}
}
