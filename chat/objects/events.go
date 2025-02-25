package objects

type EventType string

const (
	MessageReceived EventType = "message_received"
)

type Event struct {
	Event   EventType `json:"event"`
	Payload any       `json:"payload"`
}
