package stack

import (
	"sync"
)

type Context struct {
	mu sync.RWMutex
	m  map[string]interface{}
}

func NewContext() *Context {
	m := make(map[string]interface{})
	return &Context{m: m}
}

func (c *Context) Get(key string) interface{} {
	if !c.Exists(key) {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.m[key]
}

func (c *Context) Put(key string, val interface{}) *Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = val
	return c
}

func (c *Context) Delete(key string) *Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
	return c
}

func (c *Context) Exists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.m[key]
	return ok
}

func (c *Context) copy() *Context {
	nc := NewContext()
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k, v := range c.m {
		nc.m[k] = v
	}
	return nc
}
