package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"practice-run/chat/objects"
	"practice-run/guard"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

type participant struct {
	ID          uuid.UUID
	chatService *Chat
	conn        *websocket.Conn
	eventChan   *guard.ChanGuard[objects.Event]
}

func NewParticipant(conn *websocket.Conn, chatService *Chat) *participant {
	return &participant{
		ID:          uuid.New(),
		chatService: chatService,
		conn:        conn,
		eventChan:   guard.NewSafeChan[objects.Event](),
	}
}

func (p *participant) EventChan() *guard.ChanGuard[objects.Event] {
	return p.eventChan
}

func (p *participant) Run(ctx context.Context) error {
	defer p.close()
	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.Go(p.handleWS(ctx))
	errGroup.Go(p.handleEvents(ctx))
	return errGroup.Wait()
}

func (p *participant) close() {
	p.eventChan.Close()
	p.chatService.DisconnectParticipant(p)
	p.conn.Close()
}

func (p *participant) handleWS(ctx context.Context) func() error {
	return func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				var request objects.ActionRequest
				if err := p.conn.ReadJSON(&request); err != nil {
					log.Printf("failed to read message from participant %s: %v", p.ID, err)
					if websocket.IsUnexpectedCloseError(err) {
						return err
					}
					if err := p.sendError(objects.NotParsable, fmt.Sprintf("failed to read message, due: %s", err.Error())); err != nil {
						return err
					}
					continue
				}
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
	}
}

func (p *participant) handleEvents(ctx context.Context) func() error {
	eventReceiver := p.eventChan.Receiver()
	return func() error {
		for {
			select {
			case <-ctx.Done():
				p.conn.WriteJSON(objects.Event{
					Event: objects.SystemMessage,
					Payload: objects.SystemEvent{
						UserID:  p.ID,
						Message: "participant disconnected by server",
					},
				})
				return nil
			case event, ok := <-eventReceiver:
				if !ok {
					log.Printf("participant %s stopped handling messages due channel closed", p.ID)
					return nil
				}
				switch event.Event {
				case objects.MessageReceived:
					_, ok := event.Payload.(objects.MessagePayload)
					if !ok {
						log.Printf("invalid payload type (%T) for event %s", event.Payload, objects.MessageReceived)
						continue
					}
					if err := p.conn.WriteJSON(event); err != nil {
						log.Printf("failed to write message to participant %s: %v", p.ID, err)
						return err
					}
				default:
					log.Printf("unknown event type: %s", event.Event)
				}
			}
		}
	}
}

func (p *participant) handleCreateRoomRequest(request objects.ActionRequest) error {
	var room objects.RoomPayload
	if err := json.Unmarshal(request.Payload, &room); err != nil {
		if err := p.sendError(request.Action, fmt.Sprintf("failed to unmarshal room payload, due: %s", err.Error())); err != nil {
			return err
		}
		return nil
	}
	if err := p.chatService.CreateRoom(p, room); err != nil {
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
	if err := p.chatService.JoinRoom(p, room); err != nil {
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
	if err := p.chatService.LeaveRoom(p, room); err != nil {
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
	if err := p.chatService.SendMessage(p, message); err != nil {
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
	if err := p.conn.WriteJSON(objects.ActionResponse{
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
