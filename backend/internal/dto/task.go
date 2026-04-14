package dto

type CreateTaskRequest struct {
	Title       string  `json:"title" validate:"required,min=1,max=255"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	Priority    string  `json:"priority" validate:"omitempty,oneof=low medium high"`
	AssigneeID  *string `json:"assignee_id" validate:"omitempty,uuid4"`
	DueDate     *string `json:"due_date" validate:"omitempty"`
}

type UpdateTaskRequest struct {
	Title       *string `json:"title" validate:"omitempty,min=1,max=255"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	Status      *string `json:"status" validate:"omitempty,oneof=todo in_progress done"`
	Priority    *string `json:"priority" validate:"omitempty,oneof=low medium high"`
	AssigneeID  *string `json:"assignee_id" validate:"omitempty,uuid4"`
	DueDate     *string `json:"due_date" validate:"omitempty"`
	Version     int     `json:"version" validate:"required,min=1"`
}

type TaskFilterQuery struct {
	Status    string `query:"status" validate:"omitempty,oneof=todo in_progress done"`
	Assignee  string `query:"assignee" validate:"omitempty,uuid4"`
	SortBy    string `query:"sort_by" validate:"omitempty,oneof=created_at updated_at due_date priority title"`
	SortOrder string `query:"sort_order" validate:"omitempty,oneof=ASC DESC asc desc"`
	PaginationQuery
}

func (f *TaskFilterQuery) Defaults() {
	f.PaginationQuery.Defaults()
	if f.SortBy == "" {
		f.SortBy = "created_at"
	}
	if f.SortOrder == "" {
		f.SortOrder = "DESC"
	}
}
