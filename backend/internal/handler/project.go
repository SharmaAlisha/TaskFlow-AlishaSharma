package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/taskflow/backend/internal/dto"
	"github.com/taskflow/backend/internal/service"
	"github.com/taskflow/backend/internal/validator"
)

type ProjectHandler struct {
	projectService *service.ProjectService
}

func NewProjectHandler(projectService *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{projectService: projectService}
}

func (h *ProjectHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := validator.Validate(&req); err != nil {
		return err
	}

	userID := c.Locals("user_id").(string)
	project, err := h.projectService.Create(c.Context(), req, userID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(project)
}

func (h *ProjectHandler) List(c *fiber.Ctx) error {
	var pq dto.PaginationQuery
	if err := c.QueryParser(&pq); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid query parameters"})
	}
	pq.Defaults()

	userID := c.Locals("user_id").(string)
	projects, total, err := h.projectService.List(c.Context(), userID, pq)
	if err != nil {
		return err
	}

	return c.JSON(dto.PaginatedResponse[interface{}]{
		Data:       toInterfaceSlice(projects),
		Pagination: dto.NewPaginationMeta(pq.Page, pq.Limit, total),
	})
}

func (h *ProjectHandler) GetByID(c *fiber.Ctx) error {
	projectID := c.Params("id")
	userID := c.Locals("user_id").(string)

	project, tasks, err := h.projectService.GetByID(c.Context(), projectID, userID)
	if err != nil {
		return err
	}

	return c.JSON(dto.ProjectResponse{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		OwnerID:     project.OwnerID,
		CreatedAt:   project.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Tasks:       tasks,
	})
}

func (h *ProjectHandler) Update(c *fiber.Ctx) error {
	projectID := c.Params("id")

	var req dto.UpdateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := validator.Validate(&req); err != nil {
		return err
	}

	userID := c.Locals("user_id").(string)
	project, err := h.projectService.Update(c.Context(), projectID, userID, req)
	if err != nil {
		return err
	}

	return c.JSON(project)
}

func (h *ProjectHandler) Delete(c *fiber.Ctx) error {
	projectID := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := h.projectService.Delete(c.Context(), projectID, userID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ProjectHandler) Stats(c *fiber.Ctx) error {
	projectID := c.Params("id")
	userID := c.Locals("user_id").(string)

	stats, err := h.projectService.Stats(c.Context(), projectID, userID)
	if err != nil {
		return err
	}

	return c.JSON(stats)
}

func toInterfaceSlice[T any](items []T) []interface{} {
	result := make([]interface{}, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}
