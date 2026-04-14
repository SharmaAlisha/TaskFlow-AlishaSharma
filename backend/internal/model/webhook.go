package model

import "time"

type WebhookSubscription struct {
	ID         string   `json:"id"`
	UserID     string   `json:"user_id"`
	URL        string   `json:"url"`
	Secret     string   `json:"-"`
	EventTypes []string `json:"event_types"`
	ProjectIDs []string `json:"project_ids"`
	Active     bool     `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

type WebhookDeliveryLog struct {
	ID               string    `json:"id"`
	SubscriptionID   string    `json:"subscription_id"`
	EventType        string    `json:"event_type"`
	Payload          string    `json:"payload"`
	ResponseStatus   *int      `json:"response_status"`
	ResponseBody     *string   `json:"response_body"`
	Attempt          int       `json:"attempt"`
	Success          bool      `json:"success"`
	ErrorMessage     *string   `json:"error_message"`
	DeliveredAt      time.Time `json:"delivered_at"`
}
