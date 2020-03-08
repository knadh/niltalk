package hub

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type msgWrap struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type msgPeer struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
}

type msgChat struct {
	PeerID     string `json:"peer_id"`
	PeerHandle string `json:"peer_handle"`
	Msg        string `json:"message"`
}

// peerReq represents a peer request (join, leave etc.) that's processed
// by a Room.
type peerReq struct {
	reqType string
	peer    *Peer
}

// Room represents a chat room.
type Room struct {
	ID       string
	Name     string
	Password []byte
	hub      *Hub
	mut      *sync.RWMutex

	lastActivity time.Time

	// List of connected peers.
	peers map[*Peer]bool

	// Broadcast channel for messages.
	broadcastQ chan []byte

	// Peer related requests.
	peerQ chan peerReq

	// Dispose signal.
	disposeSig chan bool
	closed     bool

	// Counter for auto generated peer handles.
	counter int
}

// NewRoom returns a new instance of Room.
func NewRoom(id, name string, password []byte, h *Hub) *Room {
	return &Room{
		ID:         id,
		Name:       name,
		Password:   password,
		hub:        h,
		peers:      make(map[*Peer]bool, 100),
		broadcastQ: make(chan []byte, 100),
		peerQ:      make(chan peerReq, 100),
		disposeSig: make(chan bool),
		counter:    1,
	}
}

// AddPeer adds a new peer to the room given a WS connection from an HTTP
// handler.
func (r *Room) AddPeer(id, handle string, ws *websocket.Conn) {
	r.queuePeerReq(TypePeerJoin, newPeer(id, handle, ws, r))
}

// Dispose signals the room to notify all connected peer messages, and dispose
// of itself.
func (r *Room) Dispose() {
	r.disposeSig <- true
}

// Broadcast broadcasts a message to all connected peers.
func (r *Room) Broadcast(data []byte) {
	r.broadcastQ <- data

	// Extend the room's expiry.
	// if time.Since(r.timestamp) > time.Duration(30)*time.Second {
	// 	r.timestamp = time.Now()
	// 	r.setExpiry(config.RoomTimeout)
	// }
}

// run is a blocking function that starts the main event loop for a room that
// handles peer connection events and message broadcasts. This should be invoked
// as a goroutine.
func (r *Room) run() {
loop:
	for {
		select {
		// Dispose request.
		case <-r.disposeSig:
			break loop

		// Incoming peer request.
		case req, ok := <-r.peerQ:
			if !ok {
				break loop
			}

			switch req.reqType {
			// A new peer has joined.
			case TypePeerJoin:
				// Room's capacity is exchausted. Kick the peer out.
				if len(r.peers) > r.hub.cfg.MaxPeersPerRoom {
					req.peer.writeWSData(websocket.CloseMessage, r.makePayload("room is full", TypeNotice))
					req.peer.leave()
					continue
				}

				r.peers[req.peer] = true
				go req.peer.RunListener()
				go req.peer.RunWriter()

				// Send the peer its info.
				req.peer.SendData(r.makePeerUpdatePayload(req.peer, TypePeerInfo))

				// Notify all peers of the new addition.
				r.Broadcast(r.makePeerUpdatePayload(req.peer, TypePeerJoin))
				r.hub.log.Printf("%s@%s joined %s", req.peer.Handle, req.peer.ID, r.ID)

			// A peer has left.
			case TypePeerLeave:
				r.removePeer(req.peer)
				r.Broadcast(r.makePeerUpdatePayload(req.peer, TypePeerLeave))
				r.hub.log.Printf("%s@%s left %s", req.peer.Handle, req.peer.ID, r.ID)

			// A peer has requested the room's peer list.
			case TypePeerList:
				req.peer.SendData(r.makePeerListPayload())
			}

		// Fanout broadcast to all peers.
		case m, ok := <-r.broadcastQ:
			if !ok {
				break loop
			}
			for p := range r.peers {
				p.SendData(m)
			}

		// Kill the room after the inactivity period.
		case <-time.After(r.hub.cfg.RoomTimeout):
			break loop
		}
	}

	r.hub.log.Printf("stopped room: %v", r.ID)
	r.remove()
}

// remove disposes a room by notifying and disconnecting all peers and
// removing the room from the store.
func (r *Room) remove() {
	r.closed = true

	// Close all peer WS connections.
	for peer := range r.peers {
		peer.writeWSControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "room disposed"))
		delete(r.peers, peer)
	}

	// Close all room channels.
	close(r.broadcastQ)
	close(r.peerQ)
	r.hub.removeRoom(r.ID)
}

// queuePeerReq queues a peer addition / removal request to the room.
func (r *Room) queuePeerReq(reqType string, p *Peer) {
	if r.closed {
		return
	}
	p.room.peerQ <- peerReq{reqType: reqType, peer: p}
}

// removePeer removes a peer from the room and broadcasts a message to the
// room notifying all peers of the action.
func (r *Room) removePeer(p *Peer) {
	close(p.dataQ)
	delete(r.peers, p)

	// Notify all peers of the event.
	r.Broadcast(r.makePeerUpdatePayload(p, TypePeerLeave))
}

// sendPeerList sends the peer list to the given peer.
func (r *Room) sendPeerList(p *Peer) {
	r.peerQ <- peerReq{reqType: TypePeerList, peer: p}
}

// makePeerListPayload prepares a message payload with the list of peers.
func (r *Room) makePeerListPayload() []byte {
	peers := make([]msgPeer, 0, len(r.peers))
	for p := range r.peers {
		peers = append(peers, msgPeer{ID: p.ID, Handle: p.Handle})
	}
	return r.makePayload(peers, TypePeerList)
}

// makePeerUpdatePayload prepares a message payload representing a peer
// join / leave event.
func (r *Room) makePeerUpdatePayload(p *Peer, peerUpdateType string) []byte {
	d := msgPeer{
		ID:     p.ID,
		Handle: p.Handle,
	}
	return r.makePayload(d, peerUpdateType)
}

// makeMessagePayload prepares a chat message.
func (r *Room) makeMessagePayload(msg string, p *Peer) []byte {
	d := msgChat{
		PeerID:     p.ID,
		PeerHandle: p.Handle,
		Msg:        msg,
	}
	return r.makePayload(d, TypeMessage)
}

// makePayload prepares a message payload.
func (r *Room) makePayload(data interface{}, typ string) []byte {
	m := msgWrap{
		Timestamp: time.Now(),
		Type:      typ,
		Data:      data,
	}
	b, _ := json.Marshal(m)
	return b
}
