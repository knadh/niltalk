// Niltalk, April 2015
// https://niltalk.com
// https://github.com/goniltalk/niltalk
// License AGPL3

package main

import "testing"

func TestConnection(t *testing.T) {
	db := dbPool.Get()

	if db.conn.Err() != nil {
		t.Fatal("Redis connection failed")
	}

	defer db.Close()
}

func TestMap(t *testing.T) {
	db := dbPool.Get()
	defer db.Close()

	// Put a map.
	err := db.PutMap("nilmap", "name", "maptest", "value", 1)
	if err != nil {
		t.Fatalf("Putmap failed %v,", err)
	}

	// Retrieve the a map.
	out := struct {
		Name  string `redis:"name"`
		Value int    `redis:"value"`
	}{}

	exists, err := db.GetMap("nilmap", &out)
	if err != nil {
		t.Fatalf("Getmap failed %v,", err)
	}

	// Key doesn't exist.
	if exists == false {
		t.Fatal("nilmap hash key not found")
	}

	// Check values.
	if out.Name != "maptest" && out.Value != 1 {
		t.Fatalf("Map values don't match name=%s, value=%d", out.Name, out.Value)
	}
}

func TestSet(t *testing.T) {
	db := dbPool.Get()
	defer db.Close()

	err := db.PutSet("nilset", "val1")
	err = db.PutSet("nilset", "val2")
	if err != nil {
		t.Fatalf("PutSet failed, %v", err)
	}

	exists, err := db.SetValueExists("nilset", "val1")
	if err != nil {
		t.Fatalf("SetExists failed, %v", err)
	}

	// The set doesn't exist.
	if !exists {
		t.Error("nilset set key not found")
	}

	// Check for an inexistent value.
	exists, err = db.SetValueExists("nilset", "val3")
	if err != nil {
		t.Errorf("SetExists failed, %v", err)
	}

	if exists {
		t.Error("Inexistent 'val3' set value found")
	}
}

func TestDelete(t *testing.T) {
	db := dbPool.Get()
	defer db.Close()

	err := db.Delete("nilmap")
	err = db.Delete("nilset")
	if err != nil {
		t.Fatalf("Delete failed, %v", err)
	}

	exists, err := db.Exists("nilset")
	if err != nil {
		t.Fatalf("Exists failed, %v", err)
	}

	if exists {
		t.Fatal("Deleted key nilset exists")
	}
}
