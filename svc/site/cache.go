package site

import (
	"sync"
	"time"
)

// LISTING_CACHE_TTL bounds how stale a directory listing may be. Pages
// themselves are always read fresh; only the recursive page counts and titles
// behind a listing are cached, so edits still show up promptly.
const LISTING_CACHE_TTL = 5 * time.Second

// ttlCache is a small concurrency-safe memo with a uniform expiry.
type ttlCache[T any] struct {
	ttl     time.Duration
	mu      sync.Mutex
	entries map[string]cacheEntry[T]
}

type cacheEntry[T any] struct {
	value   T
	expires time.Time
}

func newTTLCache[T any](ttl time.Duration) *ttlCache[T] {
	return &ttlCache[T]{ttl: ttl, entries: make(map[string]cacheEntry[T])}
}

// get returns the cached value for key, computing it with build on a miss or
// an expiry. build may run more than once for the same key under contention,
// which is harmless for the pure filesystem reads this caches.
func (c *ttlCache[T]) get(key string, build func() (T, error)) (T, error) {
	now := time.Now()

	c.mu.Lock()
	entry, ok := c.entries[key]
	c.mu.Unlock()
	if ok && now.Before(entry.expires) {
		return entry.value, nil
	}

	value, err := build()
	if err != nil {
		return value, err
	}

	c.mu.Lock()
	c.entries[key] = cacheEntry[T]{value: value, expires: now.Add(c.ttl)}
	c.mu.Unlock()
	return value, nil
}
