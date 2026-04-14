package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/taskflow/backend/internal/config"
	"github.com/taskflow/backend/internal/model"
	"github.com/taskflow/backend/internal/repository"
)

type WebhookDispatcher struct {
	webhookRepo *repository.WebhookRepository
	cfg         *config.Config
	logger      *slog.Logger
	httpClient  *http.Client
	semaphore   chan struct{}
	wg          sync.WaitGroup
}

func NewWebhookDispatcher(
	webhookRepo *repository.WebhookRepository,
	cfg *config.Config,
	logger *slog.Logger,
) *WebhookDispatcher {
	return &WebhookDispatcher{
		webhookRepo: webhookRepo,
		cfg:         cfg,
		logger:      logger,
		httpClient: &http.Client{
			Timeout: cfg.WebhookTimeout,
		},
		semaphore: make(chan struct{}, cfg.WebhookWorkerPoolSz),
	}
}

func (d *WebhookDispatcher) Dispatch(evt model.Event) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		subs, err := d.webhookRepo.FindMatchingSubscriptions(ctx, string(evt.Type), evt.ProjectID)
		if err != nil {
			d.logger.Error("failed to find webhook subscriptions", "error", err)
			return
		}

		for _, sub := range subs {
			sub := sub
			d.wg.Add(1)
			go func() {
				defer d.wg.Done()
				d.semaphore <- struct{}{}
				defer func() { <-d.semaphore }()
				d.deliver(sub, evt)
			}()
		}
	}()
}

func (d *WebhookDispatcher) deliver(sub model.WebhookSubscription, evt model.Event) {
	payload := map[string]interface{}{
		"event":      string(evt.Type),
		"project_id": evt.ProjectID,
		"job_id":     evt.JobID,
		"payload":    evt.Payload,
		"timestamp":  evt.Timestamp.Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		d.logger.Error("webhook marshal error", "error", err)
		return
	}

	backoff := []time.Duration{1 * time.Second, 5 * time.Second, 25 * time.Second}

	for attempt := 0; attempt <= d.cfg.WebhookMaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff[attempt-1])
		}

		req, err := http.NewRequest("POST", sub.URL, bytes.NewReader(body))
		if err != nil {
			d.logger.Error("webhook request creation failed", "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Signature", computeSignature(sub.Secret, body))

		resp, err := d.httpClient.Do(req)
		if err != nil {
			d.logger.Warn("webhook delivery failed",
				"url", sub.URL,
				"attempt", attempt+1,
				"error", err,
			)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			d.logger.Info("webhook delivered",
				"url", sub.URL,
				"status", resp.StatusCode,
				"attempt", attempt+1,
			)
			return
		}

		d.logger.Warn("webhook non-2xx response",
			"url", sub.URL,
			"status", resp.StatusCode,
			"attempt", attempt+1,
		)
	}

	d.logger.Error("webhook delivery exhausted all retries",
		"url", sub.URL,
		"subscription_id", sub.ID,
		"event_type", evt.Type,
	)
}

func (d *WebhookDispatcher) Shutdown(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		d.logger.Info("webhook dispatcher drained")
	case <-ctx.Done():
		d.logger.Warn("webhook dispatcher shutdown timed out")
	}
}

func computeSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}
