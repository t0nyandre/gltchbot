package cache

import (
	"sync"
	"time"
)

// entry holds a cached value along with its expiration time.
type entry[V any] struct {
	value      V
	expiration time.Time
}

// TTL is a generic thread-safe TTL cache.
type TTL[K comparable, V any] struct {
	mu        sync.RWMutex
	store     map[K]entry[V]
	defaultTTL time.Duration
	stopCh    chan struct{}
	cleanupInterval time.Duration
}

// New creates a new TTL cache with the given default TTL.
// If defaultTTL <= 0, entries never expire by default (but can be set per entry).
// No background cleanup goroutine is started by default; call StartCleanup to start periodic cleanup.
func New[K comparable, V any](defaultTTL time.Duration) *TTL[K, V] {
	c := &TTL[K, V]{
		store:      make(map[K]entry[V]),
		defaultTTL: defaultTTL,
		stopCh:     make(chan struct{}),
		cleanupInterval: time.Minute,
	}
	return c
}

// Get retrieves a value by key. Returns the value and true if found and not expired.
// If the entry is expired, it is removed and the second return value is false.
func (c *TTL[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.store[key]
	c.mu.RUnlock()
	if !ok {
		var zero V
		return zero, false
	}
	if !e.expiration.IsZero() && time.Now().After(e.expiration) {
		// expired, delete it
		c.Delete(key)
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores a value with the default TTL.
func (c *TTL[K, V]) Set(key K, value V) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL stores a value with a custom TTL.
// If ttl <= 0, the entry never expires.
func (c *TTL[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.store[key] = entry[V]{
		value:      value,
		expiration: exp,
	}
	c.mu.Unlock()
}

// Delete removes a key from the cache.
func (c *TTL[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.store, key)
	c.mu.Unlock()
}

// DeleteIf removes all entries where the predicate returns true.
func (c *TTL[K, V]) DeleteIf(predicate func(K, V) bool) {
	c.mu.Lock()
	for k, e := range c.store {
		if predicate(k, e.value) {
			delete(c.store, k)
		}
	}
	c.mu.Unlock()
}

// Clear removes all entries.
func (c *TTL[K, V]) Clear() {
	c.mu.Lock()
	c.store = make(map[K]entry[V])
	c.mu.Unlock()
}

// Len returns the number of entries in the cache (including expired ones).
// Useful for debugging.
func (c *TTL[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}

// StartCleanup starts a background goroutine that periodically removes expired entries.
// The cleanup interval can be configured by setting c.cleanupInterval before calling StartCleanup.
// If the cache already has a cleanup goroutine running, this is a no-op.
func (c *TTL[K, V]) StartCleanup() {
	select {
	case <-c.stopCh:
		// channel already closed, need to recreate
		c.stopCh = make(chan struct{})
	default:
		// already running
		return
	}
	go c.cleanupLoop()
}

// Stop terminates the background cleanup goroutine.
func (c *TTL[K, V]) Stop() {
	close(c.stopCh)
}

// cleanupLoop periodically removes expired entries.
func (c *TTL[K, V]) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes all expired entries.
func (c *TTL[K, V]) cleanup() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.store {
		if !e.expiration.IsZero() && now.After(e.expiration) {
			delete(c.store, k)
		}
	}
	c.mu.Unlock()
}