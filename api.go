// Niltalk, April 2015
// https://niltalk.com
// https://github.com/goniltalk/niltalk
// License AGPL3

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/alexedwards/stack"
	"github.com/dchest/uniuri"
	"golang.org/x/crypto/bcrypt"
)

// HTML page templates.
var templates = make(map[string]*template.Template)

// HTTP handlers and endpoints.

// Homepage.
func indexPage(w http.ResponseWriter, r *http.Request) {
	c := map[string]interface{}{
		"RoomTimeout": config.RoomTimeout / 60,
		"RoomAge":     config.RoomAge / 60,
	}
	respond(w, templates["index"], c, http.StatusOK)
}

// Static pages.
func staticPage(w http.ResponseWriter, r *http.Request) {
	page_id := r.URL.Query().Get(":page_id")

	tpl, err := template.ParseFiles("templates/base.html", "pages/"+page_id+".html")
	if err == nil {
		respond(w, tpl, nil, http.StatusOK)
	} else {
		http.NotFound(w, r)
	}
}

// Chat room page.
func roomPage(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
	room := ctx.Get("room").(*Room)
	params := map[string]interface{}{
		"Room":  room,
		"Page":  "room",
		"Title": "Join #" + room.Id,
	}

	// Disable browser caching.
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	respond(w, templates["room"], params, http.StatusOK)
}

// Validate a room creation request and create the roon.
func createRoom(w http.ResponseWriter, r *http.Request) {
	password := r.FormValue("password")
	if len(password) < 5 {
		respondJSON(w, "Invalid password (min 4 chars)", nil, http.StatusBadRequest)
		return
	}

	// Hash the password.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 5)
	if err != nil {
		respondJSON(w, "Could not register password", nil, http.StatusInternalServerError)
		return
	}

	// Create the room.
	room_id, err := generateRoomId(5)

	// Couldn't generate a unique id.
	if err != nil {
		respondJSON(w, "Could not create room id", nil, http.StatusInternalServerError)
		return
	}

	err = newRoom(room_id, hash)
	if err != nil {
		respondJSON(w, err.Error(), nil, http.StatusOK)
	}

	response := struct {
		Id string `json:"id"`
	}{room_id}

	respondJSON(w, "", response, http.StatusOK)
}

// Dispose a room.
func disposeRoom(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
	room := ctx.Get("room").(*Room)

	clearSessions(room)
	room.stop <- 0

	response := struct {
		Message string `json:"message"`
	}{"Room disposed"}

	respondJSON(w, "", response, http.StatusOK)
}

// Log a peer into the room after password validation.
func login(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
	room := ctx.Get("room").(*Room)

	// Too many?
	if room.peerCount() >= config.MaxPeersPerRoom {
		respondJSON(w, "Room is full", nil, http.StatusServiceUnavailable)
		return
	}

	// Password validation.
	r.ParseForm()
	password := []byte(r.FormValue("password"))

	err := bcrypt.CompareHashAndPassword(room.password, password)

	// Password is validated.
	if err == nil {
		// Register a new session in the DB.
		token, err := newSession(room)
		if err != nil {
			clearSessions(room)
			room.stop <- 0

			respondJSON(w, "Room's gone", nil, http.StatusInternalServerError)
			return
		}

		// Session cookie for the webapp.
		c1 := &http.Cookie{Name: config.SessionCookie,
			Value: token,
			Path:  config.RoomRoute + room.Id}

		// Session cookie for Websockets.
		c2 := &http.Cookie{Name: config.SessionCookie,
			Value: token,
			Path:  config.WebsocketRoute + room.Id}

		http.SetCookie(w, c1)
		http.SetCookie(w, c2)

		// All good. Return the session token to the client.
		response := struct {
			Token string `json:"token"`
		}{token}
		respondJSON(w, "", response, http.StatusOK)

		return
	}

	respondJSON(w, "Incorrect login", nil, http.StatusForbidden)
}

// Websocket connection request handler.
func webSocketHandler(ctx *stack.Context, w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		Logger.Println("405 Method not allowed:", r.RemoteAddr, r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	room := ctx.Get("room").(*Room)

	// Peers exceeded?
	if room.peerCount() >= config.MaxPeersPerRoom {
		Logger.Println("max_peers_per_room exceeded at", config.MaxPeersPerRoom)
		http.Error(w, "Too many peers", http.StatusServiceUnavailable)
		return
	}

	// All good, upgrade to Websocket.
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		Logger.Println("Websocket upgrade failed:", r.RemoteAddr, err)
		return
	}

	// Assign a handle to the peer.
	handle := r.URL.Query().Get("handle")
	if len(handle) < 3 {
		handle = fmt.Sprintf("User%v", room.counter)
	}
	id := uniuri.NewLen(6)

	// Create a peer instance.
	peer := &Peer{sendQueue: make(chan []byte),
		ws:      ws,
		id:      id,
		handle:  handle,
		room_id: room.Id}

	// Register the peer to the room.
	room.register <- peer

	// Increment the peer count.
	room.counter += 1

	// Start the broadcaster for the peer.
	go peer.talk()

	// Send the identity information back to the peer.
	content := struct {
		Action int    `json:"a"`
		Id     string `json:"id"`
		Handle string `json:"handle"`
	}{
		TypeHandle,
		peer.id,
		peer.handle,
	}

	peer.sendQueue <- prepareMessage(content)

	// Start listener.
	peer.listen()
}

// Middleware.
// HTTP room handler (middleware) to validate all room requests.
// Attempt to get the room from 1) memory 2) the DB, if not found, fail with a 404.
func hasRoom(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			room *Room
			err  error
		)

		room_id := getRoomId(r)
		room = getRoom(room_id)

		// Room not in memory.
		if room == nil {
			room, err = initializeRoom(room_id)

			// Couldn't load room for unknown reasons.
			if err != nil {
				respondError(w, err.Error(), "", http.StatusInternalServerError)
				return
			}

			// Room doesn't exist.
			if room == nil {
				respondError(w, "Room not found", "", http.StatusNotFound)
				return
			}
		}

		// Context for chained middleware.
		ctx.Put("room", room)

		next.ServeHTTP(w, r)
	})
}

// Authenticate an http request (with cookies).
func hasAuth(ctx *stack.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		valid := false

		cookie, _ := r.Cookie(config.SessionCookie)

		// Check if it's a registered token.
		if cookie != nil {
			token := cookie.Value
			if token != "" && validateSessionToken(ctx.Get("room").(*Room), token) {
				ctx.Put("token", token)
				valid = true
			}
		}

		if !valid {
			respondJSON(w, "Not authorised", nil, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// __________
// Helpers.

// Generate a unique room id.
func generateRoomId(length int) (string, error) {
	db := dbPool.Get()
	defer db.Close()

	var id string

	// Try upto 5 times to generate a unique id.
	for i := 0; i < 5; i++ {
		id = uniuri.NewLen(length)

		exists, err := db.Exists(config.CachePrefixRoom + id)

		if err != nil {
			return "", errors.New("Unable to generate room")
		}

		// Got a unique id.
		if !exists {
			break
		}
	}

	return id, nil
}

// Get the room id from the http request.
func getRoomId(r *http.Request) string {
	return r.URL.Query().Get(":room_id")
}

// Generate a session token and register it in the DB.
func newSession(room *Room) (string, error) {
	token := uniuri.NewLen(32)

	// Add the token to room's session set.
	db := dbPool.Get()
	defer db.Close()

	err := db.PutSet(config.CachePrefixSessions+room.Id, token)

	if err == nil {
		return token, nil
	} else {
		return "", err
	}
}

// Validate a session token against tokens registered for a room.
func validateSessionToken(room *Room, token string) bool {
	db := dbPool.Get()
	defer db.Close()

	exists, err := db.SetValueExists(config.CachePrefixSessions+room.Id, token)
	if err != nil || exists == false {
		return false
	}

	return true
}

// Clear all session tokens registered to a room.
func clearSessions(room *Room) {
	db := dbPool.Get()
	defer db.Close()

	db.Delete(config.CachePrefixSessions + room.Id)
}

// Load templates and other assets.
func loadAssets() {
	templates["index"] = template.Must(template.ParseFiles("templates/base.html", "templates/index.html"))
	templates["room"] = template.Must(template.ParseFiles("templates/base.html", "templates/room.html"))
	templates["error"] = template.Must(template.ParseFiles("templates/base.html", "templates/error.html"))
}

// Prepare a json payload to send to websockets.
func prepareMessage(content interface{}) []byte {
	resp, err := json.Marshal(content)

	if err == nil {
		return resp
	}

	return []byte("")
}

// Prepare a websocket ready peers list of a room.
func preparePeersList(room *Room, peer *Peer, status bool) []byte {
	// The current peer is joining or leaving?
	var change interface{}

	if peer != nil {
		change = struct {
			Peer   string `json:"peer"`
			Handle string `json:"handle"`
			Status bool   `json:"status"`
			Time   int64  `json:"time"`
		}{
			peer.id,
			peer.handle,
			status,
			time.Now().UTC().Unix(),
		}
	}

	content := struct {
		Action  int         `json:"a"`
		Message interface{} `json:"peers"`
		Change  interface{} `json:"change"`
	}{
		TypePeers,
		room.peerList(),
		change,
	}

	return prepareMessage(content)
}

// __________
// Standard JSON http response.
func respondJSON(w http.ResponseWriter, error string, data interface{}, code int) {
	if code == 0 {
		code = http.StatusOK
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	content := struct {
		Error string      `json:"error"`
		Data  interface{} `json:"data"`
	}{
		error,
		data,
	}

	resp, err := json.Marshal(content)

	if err == nil {
		fmt.Fprint(w, string(resp))
	} else {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Standard HTML response.
func respond(w http.ResponseWriter, tpl *template.Template, context map[string]interface{}, code int) {
	if code == 0 {
		code = http.StatusOK
	}

	// Additional variables passed to the template?
	if context == nil {
		context = make(map[string]interface{})
	}

	// Pass config by default variables.
	context["Config"] = config

	// <body> class.
	if val, ok := context["Page"]; ok {
		context["Page"] = val
	} else {
		context["Page"] = ""
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tpl.Execute(w, context)
}

// Standard HTML error response.
func respondError(w http.ResponseWriter, title string, description string, code int) {
	if code == 0 {
		code = http.StatusInternalServerError
	}

	ctx := map[string]interface{}{
		"ErrorTitle":       title,
		"ErrorDescription": description,
	}

	respond(w, templates["error"], ctx, code)
}
