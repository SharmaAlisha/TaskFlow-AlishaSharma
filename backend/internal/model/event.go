package model

import "time"

type EventType string

const (
	EventTaskCreated    EventType = "task.created"
	EventTaskUpdated    EventType = "task.updated"
	EventTaskDeleted    EventType = "task.deleted"
	EventProjectUpdated EventType = "project.updated"
	EventProjectDeleted EventType = "project.deleted"
)

type Event struct {
	JobID     string      `json:"job_id"`
	Type      EventType   `json:"type"`
	ProjectID string      `json:"project_id"`
	ActorID   string      `json:"actor_id"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}
