package redis

import (
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/knadh/niltalk/store"
)

// Config represents the Redis store config structure.
type Config struct {
	Address     string        `koanf:"address"`
	Password    string        `koanf:"password"`
	DB          int           `koanf:"db"`
	ActiveConns int           `koanf:"active_conns"`
	IdleConns   int           `koanf:"idle_conns"`
	Timeout     time.Duration `koanf:"timeout"`

	PrefixRoom    string `koanf:"prefix_room"`
	PrefixSession string `koanf:"prefix_session"`
}

// Redis represents the Redis implementation of the Store interface.
type Redis struct {
	cfg  *Config
	pool *redis.Pool
}

type room struct {
	ID        string `redis:"id"`
	Name      string `redis:"name"`
	Password  []byte `redis:"password"`
	CreatedAt string `redis:"created_at"`
}

// New returns a new Redis store.
func New(cfg Config) (*Redis, error) {
	pool := &redis.Pool{
		Wait:      true,
		MaxActive: cfg.ActiveConns,
		MaxIdle:   cfg.IdleConns,
		Dial: func() (redis.Conn, error) {
			return redis.Dial(
				"tcp",
				cfg.Address,
				redis.DialPassword(cfg.Password),
				redis.DialConnectTimeout(cfg.Timeout),
				redis.DialReadTimeout(cfg.Timeout),
				redis.DialWriteTimeout(cfg.Timeout),
				redis.DialDatabase(cfg.DB),
			)
		},
	}

	// Test connection.
	c := pool.Get()
	defer c.Close()

	if err := c.Err(); err != nil {
		return nil, err
	}
	return &Redis{cfg: &cfg, pool: pool}, nil
}

// AddRoom adds a room to the store.
func (r *Redis) AddRoom(room store.Room, ttl time.Duration) error {
	c := r.pool.Get()
	defer c.Close()

	key := fmt.Sprintf(r.cfg.PrefixRoom, room.ID)
	c.Send("HMSET", key,
		"name", room.Name,
		"created_at", room.CreatedAt.Format(time.RFC3339),
		"password", room.Password)
	c.Send("EXPIRE", key, int(ttl.Seconds()))
	return c.Flush()
}

// ExtendRoomTTL extends a room's TTL.
func (r *Redis) ExtendRoomTTL(id string, ttl time.Duration) error {
	c := r.pool.Get()
	defer c.Close()

	c.Send("EXPIRE", fmt.Sprintf(r.cfg.PrefixRoom, id), int(ttl.Seconds()))
	c.Send("EXPIRE", fmt.Sprintf(r.cfg.PrefixSession, id), int(ttl.Seconds()))
	return c.Flush()
}

// GetRoom gets a room from the store.
func (r *Redis) GetRoom(id string) (store.Room, error) {
	c := r.pool.Get()
	defer c.Close()

	var (
		out  store.Room
		room room
		key  = fmt.Sprintf(r.cfg.PrefixRoom, id)
	)
	res, err := redis.Values(c.Do("HGETALL", key))
	if err != nil {
		return out, err
	}
	if err := redis.ScanStruct(res, &room); err != nil {
		return out, err
	}

	t, err := time.Parse(time.RFC3339, room.CreatedAt)
	if err != nil {
		return out, err
	}
	if t.Year() == 1 {
		return out, store.ErrRoomNotFound
	}
	return store.Room{
		ID:        id,
		Name:      room.Name,
		Password:  room.Password,
		CreatedAt: t,
	}, nil
}

// RoomExists checks if a room exists in the store.
func (r *Redis) RoomExists(id string) (bool, error) {
	c := r.pool.Get()
	defer c.Close()

	ok, err := redis.Bool(c.Do("EXISTS", fmt.Sprintf(r.cfg.PrefixRoom, id)))
	if err != nil && err != redis.ErrNil {
		return false, err
	}
	return ok, err
}

// RemoveRoom deletes a room from the store.
func (r *Redis) RemoveRoom(id string) error {
	c := r.pool.Get()
	defer c.Close()

	_, err := redis.Bool(c.Do("DEL", fmt.Sprintf(r.cfg.PrefixRoom, id)))
	return err
}

// AddSession adds a sessionID room to the store.
func (r *Redis) AddSession(sessID, handle, roomID string, ttl time.Duration) error {
	c := r.pool.Get()
	defer c.Close()

	key := fmt.Sprintf(r.cfg.PrefixSession, roomID)
	c.Send("HMSET", key, sessID, handle)
	c.Send("EXPIRE", key, ttl.Seconds)
	return c.Flush()
}

// GetSession retrieves a peer session from th store.
func (r *Redis) GetSession(sessID, roomID string) (store.Sess, error) {
	c := r.pool.Get()
	defer c.Close()

	h, err := redis.String(c.Do("HGET", fmt.Sprintf(r.cfg.PrefixSession, roomID), sessID))
	if err != nil && err != redis.ErrNil {
		return store.Sess{}, err
	}
	if h == "" {
		return store.Sess{}, nil
	}

	return store.Sess{
		ID:     sessID,
		Handle: h,
	}, nil
}

// RemoveSession deletes a session ID from a room.
func (r *Redis) RemoveSession(sessID, roomID string) error {
	c := r.pool.Get()
	defer c.Close()

	_, err := redis.Bool(c.Do("HDEL", fmt.Sprintf(r.cfg.PrefixSession, roomID), sessID))
	return err
}

// ClearSessions deletes all the sessions in a room.
func (r *Redis) ClearSessions(roomID string) error {
	c := r.pool.Get()
	defer c.Close()

	_, err := redis.Bool(c.Do("DEL", fmt.Sprintf(r.cfg.PrefixSession, roomID)))
	return err
}
