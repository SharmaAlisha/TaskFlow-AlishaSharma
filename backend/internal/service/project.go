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

type ProjectService struct {
	projectRepo *repository.ProjectRepository
	taskRepo    *repository.TaskRepository
	userRepo    *repository.UserRepository
	eventBus    *SSEBroker
	webhookDisp *WebhookDispatcher
	logger      *slog.Logger
}

func NewProjectService(
	projectRepo *repository.ProjectRepository,
	taskRepo *repository.TaskRepository,
	userRepo *repository.UserRepository,
	eventBus *SSEBroker,
	webhookDisp *WebhookDispatcher,
	logger *slog.Logger,
) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		taskRepo:    taskRepo,
		userRepo:    userRepo,
		eventBus:    eventBus,
		webhookDisp: webhookDisp,
		logger:      logger,
	}
}

func (s *ProjectService) Create(ctx context.Context, req dto.CreateProjectRequest, userID string) (*model.Project, error) {
	p := &model.Project{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     userID,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.projectRepo.Create(ctx, p); err != nil {
		return nil, apperror.NewInternal(err)
	}
	return p, nil
}

func (s *ProjectService) List(ctx context.Context, userID string, pq dto.PaginationQuery) ([]model.Project, int, error) {
	pq.Defaults()
	projects, total, err := s.projectRepo.FindByUser(ctx, userID, pq.Limit, pq.Offset())
	if err != nil {
		return nil, 0, apperror.NewInternal(err)
	}
	if projects == nil {
		projects = []model.Project{}
	}
	return projects, total, nil
}

func (s *ProjectService) GetByID(ctx context.Context, projectID, userID string) (*model.Project, []model.Task, error) {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, nil, apperror.NewInternal(err)
	}
	if p == nil {
		return nil, nil, apperror.NewNotFound("not found")
	}

	tasks, _, err := s.taskRepo.FindByProject(ctx, projectID, repository.TaskFilter{}, 1000, 0)
	if err != nil {
		return nil, nil, apperror.NewInternal(err)
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	return p, tasks, nil
}

func (s *ProjectService) Update(ctx context.Context, projectID, userID string, req dto.UpdateProjectRequest) (*model.Project, error) {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if p == nil {
		return nil, apperror.NewNotFound("not found")
	}
	if p.OwnerID != userID {
		return nil, apperror.NewForbidden("forbidden")
	}

	updated, err := s.projectRepo.Update(ctx, projectID, req.Name, req.Description)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}

	s.publishEvent(model.EventProjectUpdated, projectID, userID, updated)
	return updated, nil
}

func (s *ProjectService) Delete(ctx context.Context, projectID, userID string) error {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return apperror.NewInternal(err)
	}
	if p == nil {
		return apperror.NewNotFound("not found")
	}
	if p.OwnerID != userID {
		return apperror.NewForbidden("forbidden")
	}

	deleted, err := s.projectRepo.Delete(ctx, projectID, userID)
	if err != nil {
		return apperror.NewInternal(err)
	}
	if !deleted {
		return apperror.NewNotFound("not found")
	}

	s.publishEvent(model.EventProjectDeleted, projectID, userID, map[string]string{"id": projectID})
	return nil
}

func (s *ProjectService) Stats(ctx context.Context, projectID, userID string) (*dto.ProjectStatsResponse, error) {
	p, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if p == nil {
		return nil, apperror.NewNotFound("not found")
	}

	byStatus, err := s.taskRepo.CountByStatus(ctx, projectID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}

	byAssigneeRaw, err := s.taskRepo.CountByAssignee(ctx, projectID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}

	var byAssignee []dto.AssigneeTaskCount
	total := 0
	for _, a := range byAssigneeRaw {
		name := "Unassigned"
		if a.AssigneeID != nil {
			n, err := s.userRepo.FindNameByID(ctx, *a.AssigneeID)
			if err == nil {
				name = n
			}
		}
		byAssignee = append(byAssignee, dto.AssigneeTaskCount{
			AssigneeID: a.AssigneeID,
			Name:       name,
			Count:      a.Count,
		})
		total += a.Count
	}

	return &dto.ProjectStatsResponse{
		ProjectID:  projectID,
		TotalTasks: total,
		ByStatus:   byStatus,
		ByAssignee: byAssignee,
	}, nil
}

func (s *ProjectService) publishEvent(eventType model.EventType, projectID, actorID string, payload interface{}) {
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
