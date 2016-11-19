// Niltalk, April 2015
// https://niltalk.com
// https://github.com/goniltalk/niltalk
// License AGPL3

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// Known message/action codes in and out of Websockets.
const (
	TypeMessage = 1
	TypePeers   = 2
	TypeNotice  = 3
	TypeHandle  = 4
)

// A room.
type Room struct {
	Id        string
	password  []byte
	timestamp time.Time

	// List of connected peers.
	peers map[*Peer]bool

	// Broadcast channel for messages.
	broadcastQueue chan []byte

	// Peer registration requests.
	register chan *Peer

	// Peer removal requests.
	unregister chan *Peer

	// Stop signal.
	stop chan int

	// Counter for auto generated peer handles.
	counter int
}

// An individual peer / connection.
type Peer struct {
	// The Websocket.
	ws *websocket.Conn

	// Channel for outbound messages.
	sendQueue chan []byte

	// Peer's unique id.
	id string

	// Peer's chat handle.
	handle string

	// Peer's room id.
	room_id string

	// Rate limiting.
	numMessages int
	lastMessage time.Time
	suspended   bool
}

// Pool of active rooms.
var rooms = make(map[string]*Room)

// Create a new room.
func newRoom(id string, password []byte) error {
	// Register the room.
	db := dbPool.Get()
	defer db.Close()

	if err := db.PutMap(config.CachePrefixRoom+id, "password", password, "timestamp", time.Now()); err != nil {
		return errors.New("Unable to create room")
	}
	db.Expire(config.CachePrefixRoom+id, config.RoomAge)

	return nil
}

// Fetch a room record from the DB and initialize it in memory.
func initializeRoom(id string) (*Room, error) {
	data := struct {
		Password []byte `redis:"password"`
	}{}

	db := dbPool.Get()
	defer db.Close()

	exists, err := db.GetMap(config.CachePrefixRoom+id, &data)
	if err != nil {
		return nil, errors.New("Error loading room")
	} else if !exists {
		return nil, nil
	}

	rooms[id] = &Room{
		Id:             id,
		password:       data.Password,
		timestamp:      time.Now(),
		broadcastQueue: make(chan []byte),
		register:       make(chan *Peer),
		unregister:     make(chan *Peer),
		stop:           make(chan int),
		peers:          make(map[*Peer]bool),
		counter:        1,
	}

	// Room is loaded, start it.
	go rooms[id].run()

	return rooms[id], nil
}

// Get an initialized room by id.
func getRoom(id string) *Room {
	room, exists := rooms[id]

	if exists {
		return room
	} else {
		return nil
	}
}

// The runner loop for a room. Handles registration, deregistration, and broadcast signals.
func (room *Room) run() {
	Logger.Println("Starting room", room.Id)
	defer func() {
		Logger.Println("Stopped room", room.Id)
		room.dispose(0)
	}()

loop:
	for {
		select {
		// A new peer has connected.
		case peer, ok := <-room.register:
			if ok {
				room.peers[peer] = true

				// Notify all peers of the new addition.
				go room.broadcast(preparePeersList(room, peer, true))

				Logger.Println(fmt.Sprintf("%s@%s connected", peer.handle, peer.id))
			} else {
				break loop
			}

		// A peer has disconnected.
		case peer, ok := <-room.unregister:
			if ok {
				if _, ok := room.peers[peer]; ok {
					delete(room.peers, peer)
					close(peer.sendQueue)

					// Notify all peers of the new one.
					go room.broadcast(preparePeersList(room, peer, false))

					Logger.Println(fmt.Sprintf("%s@%s disconnected", peer.handle, peer.id))
				}
			}

		// Fanout broadcast to all peers.
		case m, ok := <-room.broadcastQueue:
			if ok {
				for peer := range room.peers {
					select {
					// Write outgoing message to the peer's send channel.
					case peer.sendQueue <- m:

					default:
						close(peer.sendQueue)
						delete(room.peers, peer)
					}
				}
			} else {
				break loop
			}

		// Stop signal.
		case _ = <-room.stop:
			break loop

		// Inactivity timeout.
		case <-time.After(time.Second * time.Duration(config.RoomTimeout)):
			break loop
		}
	}

}

// Add a message to the send queue of all peers.
func (room *Room) broadcast(data []byte) {
	if len(room.peers) > 0 {
		room.broadcastQueue <- data

		// Extend the room's age.
		if time.Since(room.timestamp) > time.Duration(30)*time.Second {
			room.timestamp = time.Now()

			room.setExpiry(config.RoomTimeout)
		}
	}
}

// All peers connected to a room.
func (room *Room) peerList() map[string]string {
	list := make(map[string]string)

	for peer := range room.peers {
		list[peer.id] = peer.handle
	}

	return list
}

// Count of peers in the room.
func (room *Room) peerCount() int {
	return len(room.peers)
}

// Sends dispose signals to all peers and deletes the room record.
func (room *Room) dispose(status int) {
	if status == 0 {
		status = websocket.CloseNormalClosure
	}

	// Close all Websocket connections.
	for peer := range room.peers {
		peer.ws.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(status, "Room disposed"),
			time.Time{})

		delete(room.peers, peer)
	}

	// Close all the room channels.
	close(room.broadcastQueue)
	close(room.register)
	close(room.unregister)

	delete(rooms, room.Id)

	// Delete the room record from the DB.
	db := dbPool.Get()
	defer db.Close()

	db.Delete(config.CachePrefixRoom + room.Id)
}

// Set the room's expiry.
func (room *Room) setExpiry(seconds int) {
	db := dbPool.Get()
	defer db.Close()

	db.Expire(config.CachePrefixRoom+room.Id, seconds)
	db.Expire(config.CachePrefixSessions+room.Id, seconds)
}

// Listen to all incoming Websocket messages.
func (peer *Peer) listen() {
	defer func() {
		if _, exists := rooms[peer.room_id]; exists {
			select {
			case rooms[peer.room_id].unregister <- peer:
			default:
			}
		}
		peer.ws.Close()
	}()

	// Maximum acceptable message length.
	peer.ws.SetReadLimit(int64(config.MaxMessageLength))

	// If there is no ping from the peer, timeout.
	pongTimeout := time.Duration(config.PongTimeout) * time.Second
	peer.ws.SetReadDeadline(time.Now().Add(pongTimeout))

	peer.ws.SetPongHandler(func(string) error {
		peer.ws.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	// This loop runs until the Websocket dies listening for incoming messages.
	for {
		_, message, err := peer.ws.ReadMessage()

		if err != nil {
			break
		}

		peer.processMessage(message)
	}
}

// Write a message to the peer's Websocket.
func (peer *Peer) write(mt int, payload []byte) error {
	writeTimeout := time.Duration(config.WriteTimeout) * time.Second

	peer.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	return peer.ws.WriteMessage(mt, payload)
}

// Process incoming messages from the room and write them to Websockets.
func (peer *Peer) talk() {
	ticker := time.NewTicker(time.Duration(config.PingTimeout) * time.Second)
	defer func() {
		ticker.Stop()
		peer.ws.Close()
	}()

	for {
		select {
		// Wait for outgoing message to appear in the channel.
		case message, ok := <-peer.sendQueue:
			if !ok {
				peer.write(websocket.CloseMessage, []byte{})
				return
			}

			// Send the message.
			if err := peer.write(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := peer.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// Disconnect a peer.
func (peer *Peer) close() {
	rooms[peer.room_id].unregister <- peer
}

// Process incoming messages.
func (peer *Peer) processMessage(message []byte) {
	data := struct {
		Action  int    `json:"a"`
		Message string `json:"m"`
	}{}

	err := json.Unmarshal(message, &data)

	// Invalid JSON.
	if err != nil || data.Action == 0 {
		return
	}

	// What action?
	switch int(data.Action) {

	// Incoming chat message.
	case TypeMessage:
		if len(data.Message) == 0 {
			return
		}

		now := time.Now()

		// Rate limiting.
		if (peer.numMessages+1)%config.RateLimitMessages == 0 &&
			time.Since(peer.lastMessage).Seconds() < config.RateLimitInterval {
			peer.close()
			return
		}

		// Rate limit counters.
		peer.lastMessage = now
		peer.numMessages += 1

		room, exists := rooms[peer.room_id]

		// Broadcast to the room.
		if exists {
			content := struct {
				Action  int         `json:"a"`
				Peer    string      `json:"p"`
				Handle  string      `json:"h"`
				Message interface{} `json:"m"`
				Time    int64       `json:"t"`
			}{
				TypeMessage,
				peer.id,
				peer.handle,
				data.Message,
				now.UTC().Unix(),
			}

			room.broadcast(prepareMessage(content))
		}

	// Request for peers list
	case TypePeers:
		room, exists := rooms[peer.room_id]

		if exists {
			peer.sendQueue <- preparePeersList(room, nil, false)
		}
	default:
		return
	}
}
