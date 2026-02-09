package cache

import (
	"sync"
	"time"
)

type Cache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]entry
}
type entry struct {
	val any
	exp time.Time
}

func New(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 5 * time.Second
	}

	return &Cache{
		ttl: ttl,
		m:   make(map[string]entry),
	}
}

func (c *Cache) Get(key string) (any, bool) {
	now := time.Now()
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}

	if now.After(e.exp) {
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		return nil, false
	}

	return e.val, true
}

func (c *Cache) Set(key string, val any) {
	c.mu.Lock()
	c.m[key] = entry{val: val, exp: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
}

func (c *Cache) Clear() {
	c.mu.Lock()
	c.m = make(map[string]entry)
	c.mu.Unlock()
}
