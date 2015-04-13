// Niltalk, April 2015
// https://niltalk.com
// https://github.com/goniltalk/niltalk
// License AGPL3

package main

import (
	"testing"
	"time"

	"github.com/alexedwards/stack"
)

func TestLogin(t *testing.T) {
	// Setup the room.
	room_id := "testroom"
	pw := "password"
	err := newRoom(room_id, []byte(pw))

	if err != nil {
		t.Fatalf("Couldn't create testroom, %v", err)
	}

	room, err := initializeRoom(room_id)
	if err != nil || room == nil || room.Id != room_id {
		t.Fatalf("Couldn't initialize testroom, %v", err)
	}

	ctx := &stack.Context{Room: room}
}

func TestGenerateRoomId(t *testing.T) {
	room_id, err := generateRoomId(5)
	if err != nil {
		t.Fatal("Couldn't generate room id")
	}
	if len(room_id) != 5 {
		t.Fatalf("Invalid room id length. 5 != %d", len(room_id))
	}

	room_id, err = generateRoomId(10)
	if err != nil {
		t.Fatal("Couldn't generate room id")
	}
	if len(room_id) != 10 {
	}
}

func TestSessions(t *testing.T) {
	r := &Room{
		Id:        "testroom",
		password:  []byte("test"),
		timestamp: time.Now(),
	}

	token, err := newSession(r)
	if err != nil {
		t.Fatalf("Couldn't register session, %v", err)
	} else {
		if len(token) != 32 {
			t.Fatalf("Invalid token string (not len 32), %s", token)
		}
	}

	// See if validation works.
	if validateSessionToken(r, token) != true {
		t.Fatal("Couldn't validate newly registered session token")
	}

	// Clear the session.
	clearSessions(r)

	if validateSessionToken(r, token) == true {
		t.Fatalf("Cleared session still persists in the db")
	}
}
