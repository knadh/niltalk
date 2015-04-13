// Niltalk, April 2015
// https://niltalk.com
// https://github.com/goniltalk/niltalk
// License AGPL3

package main

type Configuration struct {
	Address        string `json:"address"`
	Url            string `json:"url"`
	WebsocketRoute string `json:"websocket_route"`
	RoomRoute      string `json:"room_route"`

	CacheAddress        string `json:"cache_address"`
	CachePassword       string `json:"cache_password"`
	CachePoolActive     int    `json:"cache_pool_active"`
	CachePoolIdle       int    `json:"cache_pool_idle"`
	CachePrefixRoom     string `json:"cache_prefix_room"`
	CachePrefixSessions string `json:"cache_prefix_sessions"`

	PingTimeout  int `json:"ping_timeout"`
	PongTimeout  int `json:"pong_timeout"`
	WriteTimeout int `json:"write_timeout"`

	ReadBuffer       int `json:"read_buffer"`
	WriteBuffer      int `json:"write_buffer"`
	MaxMessageLength int `json:"max_message_length"`
	MaxMessageQueue  int `json:"max_message_queue"`

	RateLimitInterval float64 `json:"rate_limit_interval"`
	RateLimitMessages int     `json:"rate_limit_messages"`

	MaxRooms        int `json:"max_rooms"`
	MaxPeersPerRoom int `json:"max_peers_per_room"`
	RoomTimeout     int `json:"room_timeout"`
	RoomAge         int `json:"room_age"`

	MemoryReleaseInterval int `json:"memory_release_interval"`

	SessionCookie string `json:"session_cookie"`
	Debug         bool   `json:"debug"`
}
