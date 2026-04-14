package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/apperror"
	"github.com/taskflow/backend/internal/dto"
	"github.com/taskflow/backend/internal/model"
	"github.com/taskflow/backend/internal/repository"
)

type TaskService struct {
	taskRepo    *repository.TaskRepository
	projectRepo *repository.ProjectRepository
	eventBus    *SSEBroker
	webhookDisp *WebhookDispatcher
	logger      *slog.Logger
}

func NewTaskService(
	taskRepo *repository.TaskRepository,
	projectRepo *repository.ProjectRepository,
	eventBus *SSEBroker,
	webhookDisp *WebhookDispatcher,
	logger *slog.Logger,
) *TaskService {
	return &TaskService{
		taskRepo:    taskRepo,
		projectRepo: projectRepo,
		eventBus:    eventBus,
		webhookDisp: webhookDisp,
		logger:      logger,
	}
}

func (s *TaskService) Create(ctx context.Context, projectID string, req dto.CreateTaskRequest, userID string) (*model.Task, error) {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if p == nil {
		return nil, apperror.NewNotFound("not found")
	}

	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	now := time.Now().UTC()
	task := &model.Task{
		ID:          uuid.NewString(),
		Title:       req.Title,
		Description: req.Description,
		Status:      model.TaskStatusTodo,
		Priority:    model.TaskPriority(priority),
		ProjectID:   projectID,
		CreatorID:   userID,
		AssigneeID:  req.AssigneeID,
		DueDate:     req.DueDate,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, apperror.NewInternal(err)
	}

	s.publishEvent(model.EventTaskCreated, projectID, userID, task)
	return task, nil
}

func (s *TaskService) List(ctx context.Context, projectID string, filter dto.TaskFilterQuery, userID string) ([]model.Task, int, error) {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, 0, apperror.NewInternal(err)
	}
	if p == nil {
		return nil, 0, apperror.NewNotFound("not found")
	}

	filter.Defaults()

	repoFilter := repository.TaskFilter{
		Status:   filter.Status,
		Assignee: filter.Assignee,
	}

	tasks, total, err := s.taskRepo.FindByProject(ctx, projectID, repoFilter, filter.Limit, filter.Offset())
	if err != nil {
		return nil, 0, apperror.NewInternal(err)
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	return tasks, total, nil
}

func (s *TaskService) Update(ctx context.Context, taskID string, req dto.UpdateTaskRequest, userID string) (*model.Task, error) {
	existing, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if existing == nil {
		return nil, apperror.NewNotFound("not found")
	}

	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.AssigneeID != nil {
		updates["assignee_id"] = *req.AssigneeID
	}
	if req.DueDate != nil {
		updates["due_date"] = *req.DueDate
	}

	if len(updates) == 0 {
		return existing, nil
	}

	updated, err := s.taskRepo.Update(ctx, taskID, req.Version, updates)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if updated == nil {
		return nil, apperror.NewConflict("resource was modified by another request, please retry with the latest version")
	}

	s.publishEvent(model.EventTaskUpdated, existing.ProjectID, userID, updated)
	return updated, nil
}

func (s *TaskService) Delete(ctx context.Context, taskID, userID string) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return apperror.NewInternal(err)
	}
	if task == nil {
		return apperror.NewNotFound("not found")
	}

	p, err := s.projectRepo.FindByID(ctx, task.ProjectID)
	if err != nil {
		return apperror.NewInternal(err)
	}

	isOwner := p != nil && p.OwnerID == userID
	isCreator := task.CreatorID == userID
	if !isOwner && !isCreator {
		return apperror.NewForbidden("forbidden")
	}

	deleted, err := s.taskRepo.Delete(ctx, taskID)
	if err != nil {
		return apperror.NewInternal(err)
	}
	if !deleted {
		return apperror.NewNotFound("not found")
	}

	s.publishEvent(model.EventTaskDeleted, task.ProjectID, userID, map[string]string{"id": taskID})
	return nil
}

func (s *TaskService) publishEvent(eventType model.EventType, projectID, actorID string, payload interface{}) {
	evt := model.Event{
		JobID:     uuid.NewString(),
		Type:      eventType,
		ProjectID: projectID,
		ActorID:   actorID,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
	if s.eventBus != nil {
		s.eventBus.Publish(evt)
	}
	if s.webhookDisp != nil {
		s.webhookDisp.Dispatch(evt)
	}
}
