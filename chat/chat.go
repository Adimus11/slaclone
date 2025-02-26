package chat

import (
	"practice-run/chat/objects"
	"practice-run/guard"
	"sync"

	"github.com/google/uuid"
)

type room struct {
	mu           sync.Mutex
	participants map[uuid.UUID]*guard.ChanGuard[objects.Event]
}

func newRoom(participant *participant) *room {
	return &room{
		participants: map[uuid.UUID]*guard.ChanGuard[objects.Event]{participant.ID: participant.EventChan()},
	}
}

func (r *room) addParticipant(participant *participant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.participants[participant.ID]
	if ok {
		return ErrParticipantAlreadyInRoom
	}
	_, ok = r.participants[participant.ID]
	if ok {
		return ErrParticipantAlreadyInRoom
	}
	r.participants[participant.ID] = participant.EventChan()
	return nil
}

func (r *room) removeParticipant(participant *participant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.participants[participant.ID]
	if !ok {
		return ErrParticipantNotInRoom
	}
	delete(r.participants, participant.ID)
	return nil
}

func (r *room) broadcastMessage(message objects.MessagePayload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.participants[message.From]
	if !ok {
		return ErrParticipantNotInRoom
	}
	for id, ch := range r.participants {
		if id == message.From {
			continue
		}
		ch.Send(objects.Event{
			Event:   objects.MessageReceived,
			Payload: message,
		})
	}
	return nil
}

type Chat struct {
	rooms map[string]*room
	mu    sync.Mutex
}

func NewChat() *Chat {
	return &Chat{
		rooms: make(map[string]*room),
	}
}

func (c *Chat) CreateRoom(participant *participant, room objects.RoomPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.rooms[room.Name]
	if ok {
		return ErrRoomExists
	}
	c.rooms[room.Name] = newRoom(participant)
	return nil
}

func (c *Chat) JoinRoom(participant *participant, room objects.RoomPayload) error {
	c.mu.Lock()
	r, ok := c.rooms[room.Name]
	c.mu.Unlock()
	if !ok {
		return ErrRoomNotFound
	}
	return r.addParticipant(participant)
}

func (c *Chat) LeaveRoom(participant *participant, room objects.RoomPayload) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.rooms[room.Name]
	if !ok {
		return ErrRoomNotFound
	}
	if err := r.removeParticipant(participant); err != nil {
		return err
	}
	if len(r.participants) == 0 {
		delete(c.rooms, room.Name)
	}
	return nil
}

func (c *Chat) SendMessage(participant *participant, message objects.MessagePayload) error {
	c.mu.Lock()
	r, ok := c.rooms[message.Room]
	c.mu.Unlock()
	if !ok {
		return ErrRoomNotFound
	}
	message.From = participant.ID
	return r.broadcastMessage(message)
}

func (c *Chat) DisconnectParticipant(participant *participant) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, r := range c.rooms {
		r.removeParticipant(participant)
		if len(r.participants) == 0 {
			delete(c.rooms, name)
		}
	}
}
