package objects

import "github.com/google/uuid"

type EventType string

const (
	MessageReceived EventType = "message_received"
	SystemMessage   EventType = "system_message"
)

type Event struct {
	Event   EventType `json:"event"`
	Payload any       `json:"payload"`
}

type SystemEvent struct {
	UserID  uuid.UUID `json:"actorID"`
	Message string    `json:"message"`
}
