package store

import (
	"errors"
	"time"
)

// Store represents a backend store.
type Store interface {
	AddRoom(r Room, ttl time.Duration) error
	GetRoom(id string) (Room, error)
	ExtendRoomTTL(id string, ttl time.Duration) error
	RoomExists(id string) (bool, error)
	RemoveRoom(id string) error

	AddSession(sessID, roomID string, ttl time.Duration) error
	SessionExists(sessID, roomID string) (bool, error)
	RemoveSession(roomID, sessID string) error
	ClearSessions(roomID string) error
}

// Room represents the properties of a room in the store.
type Room struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Password  []byte    `json:"password"`
	CreatedAt time.Time `json:"created_at"`
}

// ErrRoomNotFound indicates that the requested room was not found.
var ErrRoomNotFound = errors.New("room not found")
