package dto

type CreateWebhookRequest struct {
	URL        string   `json:"url" validate:"required,url"`
	Secret     string   `json:"secret" validate:"required,min=16,max=256"`
	EventTypes []string `json:"event_types" validate:"required,min=1,dive,oneof=task.created task.updated task.deleted project.updated project.deleted"`
	ProjectIDs []string `json:"project_ids" validate:"required,min=1,dive,uuid4"`
}

type WebhookResponse struct {
	ID         string   `json:"id"`
	URL        string   `json:"url"`
	EventTypes []string `json:"event_types"`
	ProjectIDs []string `json:"project_ids"`
	Active     bool     `json:"active"`
	CreatedAt  string   `json:"created_at"`
}
