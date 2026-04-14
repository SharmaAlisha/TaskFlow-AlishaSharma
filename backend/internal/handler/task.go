package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/taskflow/backend/internal/dto"
	"github.com/taskflow/backend/internal/service"
	"github.com/taskflow/backend/internal/validator"
)

type TaskHandler struct {
	taskService *service.TaskService
}

func NewTaskHandler(taskService *service.TaskService) *TaskHandler {
	return &TaskHandler{taskService: taskService}
}

func (h *TaskHandler) Create(c *fiber.Ctx) error {
	projectID := c.Params("id")
	var req dto.CreateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := validator.Validate(&req); err != nil {
		return err
	}

	userID := c.Locals("user_id").(string)
	task, err := h.taskService.Create(c.Context(), projectID, req, userID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

func (h *TaskHandler) List(c *fiber.Ctx) error {
	projectID := c.Params("id")
	var filter dto.TaskFilterQuery
	if err := c.QueryParser(&filter); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid query parameters"})
	}
	filter.Defaults()

	userID := c.Locals("user_id").(string)
	tasks, total, err := h.taskService.List(c.Context(), projectID, filter, userID)
	if err != nil {
		return err
	}

	return c.JSON(dto.PaginatedResponse[interface{}]{
		Data:       toInterfaceSliceT(tasks),
		Pagination: dto.NewPaginationMeta(filter.Page, filter.Limit, total),
	})
}

func (h *TaskHandler) Update(c *fiber.Ctx) error {
	taskID := c.Params("id")
	var req dto.UpdateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := validator.Validate(&req); err != nil {
		return err
	}

	userID := c.Locals("user_id").(string)
	task, err := h.taskService.Update(c.Context(), taskID, req, userID)
	if err != nil {
		return err
	}

	return c.JSON(task)
}

func (h *TaskHandler) Delete(c *fiber.Ctx) error {
	taskID := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := h.taskService.Delete(c.Context(), taskID, userID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func toInterfaceSliceT[T any](items []T) []interface{} {
	result := make([]interface{}, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}
