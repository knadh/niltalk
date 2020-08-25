package upload

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Config represents the file upload options.
type Config struct {
	MaxMemory       string `koanf:"max-memory"`
	MaxUploadSize   string `koanf:"max-upload-size"`
	MaxAge          string `koanf:"max-age"`
	RateLimitPeriod string `koanf:"rate-limit-period"`
	RateLimitCount  string `koanf:"rate-limit-count"`
	RateLimitBurst  string `koanf:"rate-limit-burst"`
}

// Store file uploads in memory.
type Store struct {
	cfg    Config
	maxMem int64
	mu     sync.Mutex
	items  map[string]File
	size   int64
}

// File represents an upload.
type File struct {
	CreatedAt time.Time
	Data      []byte
	ID        string
	Name      string
	MimeType  string
}

// New returns a new file uplod store.
func New(cfg Config, maxMemory int64) *Store {
	return &Store{
		cfg:    cfg,
		maxMem: maxMemory,
		items:  make(map[string]File),
	}
}

// Add a new item to the store.
func (s *Store) Add(name, mimeType string, data []byte) (File, error) {
	h := sha1.New()
	h.Write(data)
	id := fmt.Sprintf("%x", h.Sum(nil))
	s.mu.Lock()
	defer s.mu.Unlock()
	up, ok := s.items[id]
	if ok {
		return up, nil
	}
	up.CreatedAt = time.Now()
	up.ID = id
	up.Name = name
	up.MimeType = mimeType
	up.Data = make([]byte, len(data), len(data))
	copy(up.Data, data)
	s.items[id] = up
	s.size += int64(len(data))
	for s.size > s.maxMem {
		var oldest *File
		for _, up := range s.items {
			if oldest == nil {
				oldest = &up
			} else if up.CreatedAt.Before(oldest.CreatedAt) {
				oldest = &up
			}
		}
		if oldest != nil {
			s.size -= int64(len(oldest.Data))
			delete(s.items, oldest.ID)
		}
	}
	if len(s.items) < 1 {
		return up, ErrFileTooLarge
	}
	return up, nil
}

// Get the file with given id.
func (s *Store) Get(id string) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	up, ok := s.items[id]
	if !ok {
		return up, ErrFileNotFound
	}
	return up, nil
}

// ErrFileNotFound indicates that the requested file was not found.
var ErrFileNotFound = errors.New("file not found")

// ErrFileTooLarge indicates that the file was too large.
var ErrFileTooLarge = errors.New("file too large")
