package objects

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Action string

const (
	CreateRoom  Action = "create_room"
	JoinRoom    Action = "join_room"
	LeaveRoom   Action = "leave_room"
	SendMessage Action = "send_message"
	NotParsable Action = "not_parsable"
)

type ActionRequest struct {
	Action  Action          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type RoomPayload struct {
	Name string `json:"name"`
}

type MessagePayload struct {
	Room    string    `json:"room"`
	From    uuid.UUID `json:"from,omitempty"`
	Message string    `json:"message"`
}

type ActionResponse struct {
	Action          Action `json:"action"`
	ResponseMessage string `json:"responseMessage"`
	Succeeded       bool   `json:"success"`
}
