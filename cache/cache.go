package cache

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

var ProfileCache = NewQueryCache(2 * time.Minute)

type CacheEntry struct {
	Data      any
	ExpiresAt time.Time
}

type QueryCache struct {
	mu      sync.RWMutex
	entries map[string]CacheEntry
	ttl     time.Duration
}

func NewQueryCache(ttl time.Duration) *QueryCache {
	c := &QueryCache{
		entries: make(map[string]CacheEntry),
		ttl:     ttl,
	}
	go c.evict()
	return c
}

func (c *QueryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Data, true
}

func (c *QueryCache) Set(key string, data any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

func (c *QueryCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]CacheEntry)
}

func (c *QueryCache) evict() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		c.mu.Lock()
		for key, entry := range c.entries {
			if time.Now().After(entry.ExpiresAt) {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

func BuildCacheKey(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := ""
	for _, k := range keys {
		v := params[k]
		if v != "" {
			result += fmt.Sprintf("%s=%s;", k, v)
		}
	}
	return result
}
