package webfetch

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// CacheEntry holds a cached HTTP response.
type CacheEntry struct {
	Body        string
	ContentType string
	StatusCode  int
	FetchedAt   time.Time
	TTL         time.Duration
}

// IsExpired reports whether the cache entry has expired.
func (e *CacheEntry) IsExpired() bool {
	if e.TTL <= 0 {
		return false
	}
	return time.Since(e.FetchedAt) > e.TTL
}

// FetchCache is an in-memory LRU-like cache for webfetch results.
type FetchCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	maxSize int
}

// NewFetchCache creates a cache with the given maximum number of entries.
func NewFetchCache(maxSize int) *FetchCache {
	if maxSize <= 0 {
		maxSize = 256
	}
	return &FetchCache{
		entries: make(map[string]*CacheEntry, maxSize),
		maxSize: maxSize,
	}
}

// Get returns a cached entry for the URL, or nil if absent/expired.
func (c *FetchCache) Get(url string) *CacheEntry {
	key := cacheKey(url)
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || e.IsExpired() {
		return nil
	}
	return e
}

// Set stores a cache entry. Evicts one random entry if at capacity.
func (c *FetchCache) Set(url string, entry *CacheEntry) {
	key := cacheKey(url)
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.maxSize {
		// Evict one entry (simple strategy: first key found).
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}
	c.entries[key] = entry
}

// Invalidate removes the entry for url.
func (c *FetchCache) Invalidate(url string) {
	c.mu.Lock()
	delete(c.entries, cacheKey(url))
	c.mu.Unlock()
}

// Clear removes all entries.
func (c *FetchCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*CacheEntry, c.maxSize)
	c.mu.Unlock()
}

// Size returns the current number of cached entries.
func (c *FetchCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func cacheKey(url string) string {
	h := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", h[:8])
}
