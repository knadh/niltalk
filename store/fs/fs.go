package fs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/knadh/niltalk/store"
)

// Config represents the file store config structure.
type Config struct {
	Path string `koanf:"path"`
}

// File represents the file implementation of the Store interface.
type File struct {
	cfg   *Config
	rooms map[string]*room
	data  map[string][]byte
	mu    sync.Mutex
	dirty bool
	log   *log.Logger
}

type room struct {
	store.Room
	Sessions map[string]string
	Expire   time.Time
}

// New returns a new Redis store.
func New(cfg Config, log *log.Logger) (*File, error) {
	store := &File{
		cfg:   &cfg,
		rooms: map[string]*room{},
		data:  map[string][]byte{},
		log:   log,
	}
	err := store.load()
	go store.watch()
	return store, err
}

// watch the store to clean it up.
func (m *File) watch() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		m.cleanup()
		m.save()
	}
}

// cleanup the store to removes expired items.
func (m *File) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for id, r := range m.rooms {
		if !r.Expire.IsZero() && r.Expire.Before(now) {
			delete(m.rooms, id)
			m.dirty = true
			continue
		}
	}
}

// load the data from the file system.
func (m *File) load() error {
	if _, err := os.Stat(m.cfg.Path); os.IsExist(err) {
		x := struct {
			Rooms map[string]*room
			Data  map[string][]byte
		}{}
		var data []byte
		data, err = ioutil.ReadFile(m.cfg.Path)
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, &x)
		if err != nil {
			return err
		}
		m.rooms = x.Rooms
		m.data = x.Data
	}
	return nil
}

// save the data to the file system.
func (m *File) save() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dirty {
		data, err := json.Marshal(struct {
			Rooms map[string]*room
			Data  map[string][]byte
		}{
			Rooms: m.rooms,
			Data:  m.data,
		})
		if err == nil {
			m.dirty = false
			go func() {
				err := ioutil.WriteFile(m.cfg.Path, data, os.ModePerm)
				if err != nil {
					m.log.Printf("error writing file %q: %v", m.cfg.Path, err)
				}
			}()
		}
	}
}

// AddRoom adds a room to the store.
func (m *File) AddRoom(r store.Room, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := r.ID
	m.rooms[key] = &room{
		Room:     r,
		Expire:   r.CreatedAt.Add(ttl),
		Sessions: map[string]string{},
	}
	m.dirty = true

	return nil
}

// AddPredefinedRoom adds a room to the store.
func (m *File) AddPredefinedRoom(r store.Room) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := r.ID
	m.rooms[key] = &room{
		Room:     r,
		Sessions: map[string]string{},
	}
	m.dirty = true

	return nil
}

// ExtendRoomTTL extends a room's TTL.
func (m *File) ExtendRoomTTL(id string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[id]
	if !ok {
		return store.ErrRoomNotFound
	}

	room.Expire = room.Expire.Add(ttl)
	m.rooms[id] = room
	m.dirty = true
	return nil
}

// GetRoom gets a room from the store.
func (m *File) GetRoom(id string) (store.Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out, ok := m.rooms[id]

	if !ok {
		return out.Room, store.ErrRoomNotFound
	}
	return out.Room, nil
}

// RoomExists checks if a room exists in the store.
func (m *File) RoomExists(id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.rooms[id]

	return ok, nil
}

// RemoveRoom deletes a room from the store.
func (m *File) RemoveRoom(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rooms[id]; ok {
		delete(m.rooms, id)
		m.dirty = true
	}

	return nil
}

// AddSession adds a sessionID room to the store.
func (m *File) AddSession(sessID, handle, roomID string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]

	if !ok {
		return store.ErrRoomNotFound
	}

	room.Sessions[sessID] = handle
	m.rooms[roomID] = room
	m.dirty = true

	return nil
}

// GetSession retrieves a peer session from the store.
func (m *File) GetSession(sessID, roomID string) (store.Sess, error) {
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
func (m *File) RemoveSession(sessID, roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]

	if !ok {
		return store.ErrRoomNotFound
	}

	if _, ok := room.Sessions[sessID]; ok {
		delete(room.Sessions, sessID)
		m.rooms[roomID] = room
		m.dirty = true
	}

	return nil
}

// ClearSessions deletes all the sessions in a room.
func (m *File) ClearSessions(roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]

	if !ok {
		return store.ErrRoomNotFound
	}

	room.Sessions = map[string]string{}

	m.rooms[roomID] = room
	m.dirty = true

	return nil
}

// Get value from a key.
func (m *File) Get(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	return d, nil
}

// Set a value.
func (m *File) Set(key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = make([]byte, len(data), len(data))
	copy(m.data[key], data)
	m.dirty = true
	return nil
}
