package upload

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/alecthomas/units"
	tparse "github.com/karrick/tparse/v2"
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
	cfg   Config
	mu    sync.Mutex
	items map[string]File
	size  int64

	MaxMemory     int64
	MaxUploadSize int64
	MaxAge        time.Duration
	RlPeriod      time.Duration
	RlCount       float64
	RlBurst       int
}

//Init the store, parsing configuration values.
func (s *Store) Init() error {
	s.MaxMemory = 32 << 20
	if s.cfg.MaxMemory != "" {
		x, err := units.ParseStrictBytes(s.cfg.MaxMemory)
		if err != nil {
			return fmt.Errorf("error unmarshalling 'upload.max-memory' config: %v", err)
		}
		s.MaxMemory = x
	}

	s.MaxUploadSize = 32 << 20
	if s.cfg.MaxUploadSize != "" {
		x, err := units.ParseStrictBytes(s.cfg.MaxUploadSize)
		if err != nil {
			return fmt.Errorf("error unmarshalling 'upload.max-upload-size' config: %v", err)
		}
		s.MaxUploadSize = x
	}

	s.MaxAge = time.Hour * 24 * 30 * 12
	if s.cfg.MaxAge != "" {
		x, err := tparse.AbsoluteDuration(time.Now(), s.cfg.MaxAge)
		if err != nil {
			return fmt.Errorf("error unmarshalling 'upload.max-age' config: %v", err)
		}
		s.MaxAge = x
	}

	s.RlPeriod = time.Minute
	if s.cfg.RateLimitPeriod != "" {
		x, err := tparse.AbsoluteDuration(time.Now(), s.cfg.RateLimitPeriod)
		if err != nil {
			return fmt.Errorf("error unmarshalling 'upload.rate-limit-period' config: %v", err)
		}
		s.RlPeriod = x
	}

	s.RlCount = 20.0
	if s.cfg.RateLimitCount != "" {
		x, err := strconv.ParseFloat(s.cfg.RateLimitCount, 64)
		if err != nil {
			return fmt.Errorf("error unmarshalling 'upload.rate-limit-count' config: %v", err)
		}
		s.RlCount = x
	}

	s.RlBurst = 1
	if s.cfg.RateLimitBurst != "" {
		x, err := strconv.Atoi(s.cfg.RateLimitBurst)
		if err != nil {
			return fmt.Errorf("error unmarshalling 'upload.rate-limit-burst' config: %v", err)
		}
		s.RlBurst = x
	}
	return nil
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
func New(cfg Config) *Store {
	return &Store{
		cfg:   cfg,
		items: make(map[string]File),
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
	for s.size > s.MaxMemory {
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
