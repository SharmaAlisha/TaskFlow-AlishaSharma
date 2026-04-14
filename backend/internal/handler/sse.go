package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/model"
	"github.com/taskflow/backend/internal/repository"
	"github.com/taskflow/backend/internal/service"
)

const maxSSEConnsPerUser = 5

type SSEHandler struct {
	broker      *service.SSEBroker
	projectRepo *repository.ProjectRepository
	logger      *slog.Logger
	userConns   sync.Map // userID -> *atomic.Int32
}

func NewSSEHandler(broker *service.SSEBroker, projectRepo *repository.ProjectRepository, logger *slog.Logger) *SSEHandler {
	return &SSEHandler{
		broker:      broker,
		projectRepo: projectRepo,
		logger:      logger,
	}
}

func (h *SSEHandler) getUserConnCount(userID string) *atomic.Int32 {
	val, _ := h.userConns.LoadOrStore(userID, &atomic.Int32{})
	return val.(*atomic.Int32)
}

func (h *SSEHandler) Stream(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	projectsParam := c.Query("projects")
	if projectsParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "projects query parameter is required"})
	}
	projectIDs := strings.Split(projectsParam, ",")

	for _, pid := range projectIDs {
		inProject, err := h.projectRepo.IsUserInProject(c.Context(), pid, userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
		}
		if !inProject {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
		}
	}

	counter := h.getUserConnCount(userID)
	if counter.Load() >= maxSSEConnsPerUser {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "too many SSE connections",
		})
	}
	counter.Add(1)
	defer counter.Add(-1)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	client := &service.SSEClient{
		ID:     uuid.NewString(),
		UserID: userID,
		Events: make(chan model.Event, 64),
		Done:   make(chan struct{}),
	}

	h.broker.Subscribe(client, projectIDs)
	defer h.broker.Unsubscribe(client.ID, projectIDs)

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		_, _ = fmt.Fprintf(w, ":connected\n\n")
		if err := w.Flush(); err != nil {
			return
		}

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case evt := <-client.Events:
				data, err := json.Marshal(evt)
				if err != nil {
					h.logger.Error("SSE marshal error", "error", err)
					continue
				}
				fmt.Fprintf(w, "event: %s\nid: %s\ndata: %s\n\n", evt.Type, evt.JobID, string(data))
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				fmt.Fprintf(w, ":ping\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			case <-client.Done:
				return
			}
		}
	})

	return nil
}
