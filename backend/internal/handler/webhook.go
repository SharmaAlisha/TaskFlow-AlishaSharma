package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/apperror"
	"github.com/taskflow/backend/internal/dto"
	"github.com/taskflow/backend/internal/model"
	"github.com/taskflow/backend/internal/repository"
	"github.com/taskflow/backend/internal/validator"
)

type WebhookHandler struct {
	webhookRepo *repository.WebhookRepository
}

func NewWebhookHandler(webhookRepo *repository.WebhookRepository) *WebhookHandler {
	return &WebhookHandler{webhookRepo: webhookRepo}
}

func (h *WebhookHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := validator.Validate(&req); err != nil {
		return err
	}

	userID := c.Locals("user_id").(string)

	sub := &model.WebhookSubscription{
		ID:         uuid.NewString(),
		UserID:     userID,
		URL:        req.URL,
		Secret:     req.Secret,
		EventTypes: req.EventTypes,
		ProjectIDs: req.ProjectIDs,
		Active:     true,
		CreatedAt:  time.Now().UTC(),
	}

	if err := h.webhookRepo.Create(c.Context(), sub); err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return apperror.NewConflict("webhook subscription already exists for this URL and event type")
		}
		return apperror.NewInternal(err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto.WebhookResponse{
		ID:         sub.ID,
		URL:        sub.URL,
		EventTypes: sub.EventTypes,
		ProjectIDs: sub.ProjectIDs,
		Active:     sub.Active,
		CreatedAt:  sub.CreatedAt.Format(time.RFC3339),
	})
}

func (h *WebhookHandler) List(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	subs, err := h.webhookRepo.FindByUser(c.Context(), userID)
	if err != nil {
		return apperror.NewInternal(err)
	}

	var resp []dto.WebhookResponse
	for _, s := range subs {
		resp = append(resp, dto.WebhookResponse{
			ID:         s.ID,
			URL:        s.URL,
			EventTypes: s.EventTypes,
			ProjectIDs: s.ProjectIDs,
			Active:     s.Active,
			CreatedAt:  s.CreatedAt.Format(time.RFC3339),
		})
	}
	if resp == nil {
		resp = []dto.WebhookResponse{}
	}

	return c.JSON(fiber.Map{"data": resp})
}

func (h *WebhookHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := c.Locals("user_id").(string)

	deleted, err := h.webhookRepo.Delete(c.Context(), id, userID)
	if err != nil {
		return apperror.NewInternal(err)
	}
	if !deleted {
		return apperror.NewNotFound("not found")
	}

	return c.SendStatus(fiber.StatusNoContent)
}
