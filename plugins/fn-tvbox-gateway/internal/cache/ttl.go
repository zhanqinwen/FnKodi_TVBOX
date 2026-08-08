package cache

import (
	"sync"
	"time"
)

// TTL is a small in-memory TTL cache for marshaled JSON (or any []byte).
type TTL struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]entry
}

type entry struct {
	data      []byte
	expiresAt time.Time
}

func NewTTL(ttl time.Duration) *TTL {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &TTL{ttl: ttl, entries: make(map[string]entry)}
}

func (c *TTL) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		if ok {
			c.mu.Lock()
			delete(c.entries, key)
			c.mu.Unlock()
		}
		return nil, false
	}
	return e.data, true
}

func (c *TTL) Set(key string, data []byte) {
	cp := make([]byte, len(data))
	copy(cp, data)
	c.mu.Lock()
	c.entries[key] = entry{data: cp, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}
