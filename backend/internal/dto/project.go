package dto

import "github.com/taskflow/backend/internal/model"

type CreateProjectRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
}

type UpdateProjectRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=1,max=255"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
}

type ProjectResponse struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description *string      `json:"description"`
	OwnerID     string       `json:"owner_id"`
	CreatedAt   string       `json:"created_at"`
	Tasks       []model.Task `json:"tasks,omitempty"`
}

type ProjectStatsResponse struct {
	ProjectID    string                  `json:"project_id"`
	TotalTasks   int                     `json:"total_tasks"`
	ByStatus     map[string]int          `json:"by_status"`
	ByAssignee   []AssigneeTaskCount     `json:"by_assignee"`
}

type AssigneeTaskCount struct {
	AssigneeID *string `json:"assignee_id"`
	Name       string  `json:"name"`
	Count      int     `json:"count"`
}
