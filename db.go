// Niltalk, April 2015
// https://niltalk.com
// https://github.com/goniltalk/niltalk
// License AGPL3

package main

import (
	"errors"
	"time"

	"github.com/garyburd/redigo/redis"
)

type DBpool struct {
	pool redis.Pool
}

type DBconn struct {
	conn redis.Conn
}

// Initialize a db pool.
func NewDBpool(address string, password string, active int, idle int) *DBpool {
	pool := redis.Pool{
		MaxActive: active,
		MaxIdle:   idle,
		Dial: func() (redis.Conn, error) {
			c, err := redis.DialTimeout("tcp", address, time.Duration(5)*time.Second, time.Duration(5)*time.Second, time.Duration(5)*time.Second)
			if err != nil {
				return nil, err
			}
			if password != "" {
				c.Do("AUTH", password)
			}
			return c, err
		},
	}

	return &DBpool{pool: pool}
}

// Get a connection from the pool.
func (dp *DBpool) Get() *DBconn {
	return &DBconn{conn: dp.pool.Get()}
}

// Close the pool.
func (dp *DBpool) Close() {
	dp.pool.Close()
}

// Retrieve a map from the db.
func (db *DBconn) GetMap(key string, out interface{}) (bool, error) {
	res, err := redis.Values(db.conn.Do("HGETALL", key))

	if err != nil {
		return false, errors.New("Failed to load from DB")
	}

	// No such entry.
	if len(res) == 0 {
		return false, nil
	}

	redis.ScanStruct(res, out)

	return true, nil
}

// Write a map to the db.
func (db *DBconn) PutMap(args ...interface{}) error {
	_, err := db.conn.Do("HMSET", args...)

	return err
}

// Write a set to the db.
func (db *DBconn) PutSet(args ...interface{}) error {
	_, err := db.conn.Do("SADD", args...)

	return err
}

// Set a key's expiry.
func (db *DBconn) Expire(key string, seconds int) error {
	_, err := db.conn.Do("EXPIRE", key, seconds)

	return err
}

// Delete a key.
func (db *DBconn) Delete(key string) error {
	_, err := db.conn.Do("DEL", key)

	return err
}

// Check if a key exists in the db.
func (db *DBconn) Exists(key string) (bool, error) {
	exists, err := redis.Bool(db.conn.Do("EXISTS", key))

	if err != nil {
		return false, err
	}

	return exists, nil
}

// Check if a value exists in a set.
func (db *DBconn) SetValueExists(key string, value string) (bool, error) {
	exists, err := redis.Bool(db.conn.Do("SISMEMBER", key, value))

	if err != nil {
		return false, err
	}

	return exists, nil
}

// Close a connection.
func (db *DBconn) Close() {
	db.conn.Close()
}
