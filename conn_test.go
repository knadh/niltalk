// Niltalk, April 2015
// https://niltalk.com
// https://github.com/goniltalk/niltalk
// License AGPL3

package main

import "testing"

func TestRoom(t *testing.T) {
	room_id := "testroom"
	pw := "password"
	err := newRoom(room_id, []byte(pw))

	if err != nil {
		t.Fatalf("Couldn't create testroom, %v", err)
	}

	data := struct {
		Password string `redis:"password"`
	}{}

	// Check if the room's been registered.
	db := dbPool.Get()
	defer db.Close()

	exists, err := db.GetMap(config.CachePrefixRoom+room_id, &data)
	if err != nil {
		t.Fatalf("Getmap failed %v,", err)
	}

	if !exists || data.Password != pw {
		t.Fatal("Couldn't retrieve newly created room from Redis")
	}
}

func TestInitializeRoom(t *testing.T) {
	room_id := "testroom"

	room, err := initializeRoom(room_id)
	if err != nil || room == nil || room.Id != room_id {
		t.Fatalf("Couldn't initialize testroom, %v", err)
	}

	r := getRoom(room_id)
	if r != room {
		t.Fatal("Couldn't get initialized testroom")
	}

	// Send the dispose signal.
	r.stop <- 0
}

func TestDisposedRoom(t *testing.T) {
	room_id := "testroom"

	db := dbPool.Get()
	defer db.Close()

	exists, err := db.Exists(config.CachePrefixRoom + room_id)
	if err != nil {
		t.Fatalf("Exists failed %v,", err)
	}
	if exists {
		t.Fatal("Disposed testroom exists in db")
	}
}
