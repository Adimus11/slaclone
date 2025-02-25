package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"practice-run/chat/objects"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

type participant struct {
	ChatService   *Chat
	Conn          *websocket.Conn
	ID            uuid.UUID
	eventReceiver chan objects.Event
}

func NewParticipant(conn *websocket.Conn, chatService *Chat) *participant {
	return &participant{
		ChatService:   chatService,
		Conn:          conn,
		ID:            uuid.New(),
		eventReceiver: make(chan objects.Event),
	}
}

func (p *participant) HandleMessages(ctx context.Context, errGroup *errgroup.Group) {
	errGroup.Go(func() error {
		getRequestChan, errChan, close := p.handleWSInput()
		defer close()

		for {
			select {
			case <-ctx.Done():
				return nil
			case err := <-errChan:
				return err
			case request := <-getRequestChan:
				switch request.Action {
				case objects.CreateRoom:
					if err := p.handleCreateRoomRequest(request); err != nil {
						return err
					}
				case objects.JoinRoom:
					if err := p.handleJoinRoomRequest(request); err != nil {
						return err
					}
				case objects.LeaveRoom:
					if err := p.handleLeaveRoomRequest(request); err != nil {
						return err
					}
				case objects.SendMessage:
					if err := p.handleSendMessageRequest(request); err != nil {
						return err
					}
				default:
					if err := p.sendError(objects.NotParsable, fmt.Sprintf("unknown action type: %s", request.Action)); err != nil {
						return err
					}
				}
			}
		}
	})
}

func (p *participant) HandleEvents(ctx context.Context, errGroup *errgroup.Group) {
	errGroup.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case event := <-p.eventReceiver:
				switch event.Event {
				case objects.MessageReceived:
					_, ok := event.Payload.(objects.MessagePayload)
					if !ok {
						log.Printf("invalid payload type (%T) for event %s", event.Payload, objects.MessageReceived)
						continue
					}
					if err := p.Conn.WriteJSON(event); err != nil {
						log.Printf("failed to write message to participant %s: %v", p.ID, err)
						return err
					}
				default:
					log.Printf("unknown event type: %s", event.Event)
				}
			}
		}
	})
}

func (p *participant) Close() {
	p.ChatService.DisconnectParticipant(p)
	close(p.eventReceiver)
	p.Conn.Close()
}

func (p *participant) handleCreateRoomRequest(request objects.ActionRequest) error {
	var room objects.RoomPayload
	if err := json.Unmarshal(request.Payload, &room); err != nil {
		if err := p.sendError(request.Action, fmt.Sprintf("failed to unmarshal room payload, due: %s", err.Error())); err != nil {
			return err
		}
		return nil
	}
	if err := p.ChatService.CreateRoom(p, room); err != nil {
		if err := p.sendError(request.Action, fmt.Sprintf("failed to create room, due: %s", err.Error())); err != nil {
			return err
		}
		return nil
	}
	return p.sendSuccess(request.Action, fmt.Sprintf("room (%s) created successfully", room.Name))
}

func (p *participant) handleJoinRoomRequest(request objects.ActionRequest) error {
	var room objects.RoomPayload
	if err := json.Unmarshal(request.Payload, &room); err != nil {
		if err := p.sendError(request.Action, fmt.Sprintf("failed to unmarshal room payload, due: %s", err.Error())); err != nil {
			return err
		}
		return nil
	}
	if err := p.ChatService.JoinRoom(p, room); err != nil {
		if err := p.sendError(request.Action, fmt.Sprintf("failed to join room, due: %s", err.Error())); err != nil {
			return err
		}
		return nil
	}
	return p.sendSuccess(request.Action, fmt.Sprintf("room (%s) joined successfully", room.Name))
}

func (p *participant) handleLeaveRoomRequest(request objects.ActionRequest) error {
	var room objects.RoomPayload
	if err := json.Unmarshal(request.Payload, &room); err != nil {
		if err := p.sendError(request.Action, fmt.Sprintf("failed to unmarshal room payload, due: %s", err.Error())); err != nil {
			return err
		}
		return nil
	}
	if err := p.ChatService.LeaveRoom(p, room); err != nil {
		if err := p.sendError(request.Action, fmt.Sprintf("failed to leave room, due: %s", err.Error())); err != nil {
			return err
		}
		return nil
	}
	return p.sendSuccess(request.Action, fmt.Sprintf("room (%s) left successfully", room.Name))
}

func (p *participant) handleSendMessageRequest(request objects.ActionRequest) error {
	var message objects.MessagePayload
	if err := json.Unmarshal(request.Payload, &message); err != nil {
		if err := p.sendError(request.Action, fmt.Sprintf("failed to unmarshal message payload, due: %s", err.Error())); err != nil {
			return err
		}
		return nil
	}
	if err := p.ChatService.SendMessage(p, message); err != nil {
		if err := p.sendError(request.Action, fmt.Sprintf("failed to send message, due: %s", err.Error())); err != nil {
			return err
		}
		return nil
	}
	return p.sendSuccess(request.Action, "message sent successfully")
}

func (p *participant) sendError(actionType objects.Action, reason string) error {
	return p.sendResponse(actionType, reason, false)
}

func (p *participant) sendSuccess(actionType objects.Action, reason string) error {
	return p.sendResponse(actionType, reason, true)
}

func (p *participant) sendResponse(actionType objects.Action, reason string, success bool) error {
	if err := p.Conn.WriteJSON(objects.ActionResponse{
		Action:          actionType,
		ResponseMessage: reason,
		Succeeded:       success,
	}); err != nil {
		log.Printf("failed to write response to participant %s: %v", p.ID, err)
		if websocket.IsUnexpectedCloseError(err) {
			return err
		}
	}
	return nil
}

func (p *participant) handleWSInput() (<-chan objects.ActionRequest, <-chan error, func()) {
	getRequestChan := make(chan objects.ActionRequest)
	errChan := make(chan error)
	go func() {
		for {
			var request objects.ActionRequest
			if err := p.Conn.ReadJSON(&request); err != nil {
				log.Printf("failed to read message from participant %s: %v", p.ID, err)
				if websocket.IsUnexpectedCloseError(err) {
					errChan <- err
					return
				}
				if err := p.sendError(objects.NotParsable, fmt.Sprintf("failed to read message, due: %s", err.Error())); err != nil {
					errChan <- err
					return
				}
			}
			getRequestChan <- request
		}
	}()

	return getRequestChan, errChan, func() {
		close(getRequestChan)
		close(errChan)
	}
}
