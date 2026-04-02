package engine

import (
	"sync"
)

// FileStateCache is an LRU cache of file contents read by tools during a
// session.  It avoids redundant disk reads when the same file is accessed
// multiple times within a conversation turn.
//
// Aligned with claude-code-main's readFileState / FileStateCache.
type FileStateCache struct {
	mu       sync.RWMutex
	entries  map[string]*fileCacheEntry
	order    []string // LRU order (most-recently-used at end)
	maxItems int
}

type fileCacheEntry struct {
	Content  string
	Hash     string
	SizeBytes int
	ModTime  int64 // unix millis
}

// NewFileStateCache creates a cache with the given capacity.
func NewFileStateCache(maxItems int) *FileStateCache {
	if maxItems <= 0 {
		maxItems = 200
	}
	return &FileStateCache{
		entries:  make(map[string]*fileCacheEntry),
		order:    make([]string, 0, maxItems),
		maxItems: maxItems,
	}
}

// Get returns the cached content and hash for path, or ("", "", false).
func (c *FileStateCache) Get(path string) (content, hash string, ok bool) {
	c.mu.RLock()
	e, exists := c.entries[path]
	c.mu.RUnlock()
	if !exists {
		return "", "", false
	}
	// Promote to MRU.
	c.mu.Lock()
	c.promote(path)
	c.mu.Unlock()
	return e.Content, e.Hash, true
}

// Has reports whether path is cached.
func (c *FileStateCache) Has(path string) bool {
	c.mu.RLock()
	_, ok := c.entries[path]
	c.mu.RUnlock()
	return ok
}

// Set stores content for path, evicting the LRU entry if at capacity.
func (c *FileStateCache) Set(path, content, hash string, sizeBytes int, modTime int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[path]; exists {
		c.entries[path] = &fileCacheEntry{Content: content, Hash: hash, SizeBytes: sizeBytes, ModTime: modTime}
		c.promote(path)
		return
	}

	// Evict LRU if full.
	if len(c.order) >= c.maxItems {
		evict := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, evict)
	}

	c.entries[path] = &fileCacheEntry{Content: content, Hash: hash, SizeBytes: sizeBytes, ModTime: modTime}
	c.order = append(c.order, path)
}

// Delete removes a path from the cache.
func (c *FileStateCache) Delete(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, path)
	for i, p := range c.order {
		if p == path {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// Clear removes all entries.
func (c *FileStateCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*fileCacheEntry)
	c.order = c.order[:0]
}

// Len returns the number of cached entries.
func (c *FileStateCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// promote moves path to the end of the LRU order (caller must hold mu).
func (c *FileStateCache) promote(path string) {
	for i, p := range c.order {
		if p == path {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, path)
			return
		}
	}
}
