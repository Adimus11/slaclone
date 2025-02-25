package chat

import "errors"

var (
	ErrRoomExists               = errors.New("room already exists")
	ErrRoomNotFound             = errors.New("room not found")
	ErrParticipantNotInRoom     = errors.New("participant not in room")
	ErrParticipantAlreadyInRoom = errors.New("participant already in room")
)
