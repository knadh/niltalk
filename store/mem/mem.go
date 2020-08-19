package mem

import (
	"fmt"
	"sync"
	"time"

	"github.com/knadh/niltalk/store"
)

// Config represents the InMemory store config structure.
type Config struct{}

// InMemory represents the in-memory implementation of the Store interface.
type InMemory struct {
	cfg   *Config
	rooms map[string]*room
	data  map[string][]byte
	mu    sync.Mutex
}

type room struct {
	store.Room
	Sessions map[string]string
	Expire   time.Time
}

// New returns a new Redis store.
func New(cfg Config) (*InMemory, error) {
	store := &InMemory{
		cfg:   &cfg,
		rooms: map[string]*room{},
		data:  map[string][]byte{},
	}
	go store.watch()
	return store, nil
}

// watch the store to clean it up.
func (m *InMemory) watch() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		m.cleanup()
	}
}

// cleanup the store to removes expired items.
func (m *InMemory) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for id, r := range m.rooms {
		if r.Expire.Before(now) {
			delete(m.rooms, id)
			continue
		}
	}
}

// AddRoom adds a room to the store.
func (m *InMemory) AddRoom(r store.Room, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rooms[r.ID] = &room{
		Room:     r,
		Expire:   r.CreatedAt.Add(ttl),
		Sessions: map[string]string{},
	}

	return nil
}

// ExtendRoomTTL extends a room's TTL.
func (m *InMemory) ExtendRoomTTL(id string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[id]
	if !ok {
		return store.ErrRoomNotFound
	}

	room.Expire = room.Expire.Add(ttl)
	m.rooms[id] = room
	return nil
}

// GetRoom gets a room from the store.
func (m *InMemory) GetRoom(id string) (store.Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out, ok := m.rooms[id]

	if !ok {
		return out.Room, store.ErrRoomNotFound
	}
	return out.Room, nil
}

// RoomExists checks if a room exists in the store.
func (m *InMemory) RoomExists(id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.rooms[id]

	return ok, nil
}

// RemoveRoom deletes a room from the store.
func (m *InMemory) RemoveRoom(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.rooms, id)

	return nil
}

// AddSession adds a sessionID room to the store.
func (m *InMemory) AddSession(sessID, handle, roomID string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]

	if !ok {
		return store.ErrRoomNotFound
	}

	room.Sessions[sessID] = handle
	m.rooms[roomID] = room

	return nil
}

// GetSession retrieves a peer session from the store.
func (m *InMemory) GetSession(sessID, roomID string) (store.Sess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]

	if !ok {
		return store.Sess{}, store.ErrRoomNotFound
	}

	handle, ok := room.Sessions[sessID]

	if !ok {
		return store.Sess{}, nil
	}

	return store.Sess{
		ID:     sessID,
		Handle: handle,
	}, nil
}

// RemoveSession deletes a session ID from a room.
func (m *InMemory) RemoveSession(sessID, roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]

	if !ok {
		return store.ErrRoomNotFound
	}

	delete(room.Sessions, sessID)
	m.rooms[roomID] = room

	return nil
}

// ClearSessions deletes all the sessions in a room.
func (m *InMemory) ClearSessions(roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]

	if !ok {
		return store.ErrRoomNotFound
	}

	room.Sessions = map[string]string{}

	m.rooms[roomID] = room

	return nil
}

// Get value from a key.
func (m *InMemory) Get(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	return d, nil
}

// Set a value.
func (m *InMemory) Set(key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = make([]byte, len(data), len(data))
	copy(m.data[key], data)
	return nil
}
