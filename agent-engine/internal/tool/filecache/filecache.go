package filecache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sync"
	"time"
)

// Entry holds cached metadata and content for a single file.
type Entry struct {
	Path       string
	Content    string
	Hash       string // SHA-256 hex
	ModTime    time.Time
	Size       int64
	CachedAt   time.Time
	Stale      bool // true if on-disk file has changed since caching
}

// Cache is a thread-safe in-memory cache of file contents and metadata,
// used by tools to avoid re-reading unchanged files and to detect
// external modifications.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	maxSize int // max number of entries
}

// New creates a file cache with the given capacity.
func New(maxEntries int) *Cache {
	if maxEntries <= 0 {
		maxEntries = 500
	}
	return &Cache{
		entries: make(map[string]*Entry, maxEntries),
		maxSize: maxEntries,
	}
}

// Get returns a cached entry if it exists and is still fresh.
// Returns nil if not cached or stale.
func (c *Cache) Get(path string) *Entry {
	c.mu.RLock()
	e, ok := c.entries[path]
	c.mu.RUnlock()
	if !ok {
		return nil
	}

	// Check if file changed on disk.
	info, err := os.Stat(path)
	if err != nil {
		c.Invalidate(path)
		return nil
	}
	if info.ModTime().After(e.ModTime) || info.Size() != e.Size {
		c.mu.Lock()
		e.Stale = true
		c.mu.Unlock()
		return nil
	}
	return e
}

// Put stores file content in the cache.
func (c *Cache) Put(path, content string) *Entry {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	hash := sha256.Sum256([]byte(content))
	entry := &Entry{
		Path:     path,
		Content:  content,
		Hash:     fmt.Sprintf("%x", hash),
		ModTime:  info.ModTime(),
		Size:     info.Size(),
		CachedAt: time.Now(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity.
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}
	c.entries[path] = entry
	return entry
}

// Invalidate removes a path from the cache.
func (c *Cache) Invalidate(path string) {
	c.mu.Lock()
	delete(c.entries, path)
	c.mu.Unlock()
}

// InvalidateAll clears the entire cache.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	c.entries = make(map[string]*Entry, c.maxSize)
	c.mu.Unlock()
}

// Has reports whether a fresh entry exists for the path.
func (c *Cache) Has(path string) bool {
	return c.Get(path) != nil
}

// Hash returns the content hash for a cached file, or "" if not cached.
func (c *Cache) Hash(path string) string {
	e := c.Get(path)
	if e == nil {
		return ""
	}
	return e.Hash
}

// IsModified checks if the file at path has been modified since it was cached.
func (c *Cache) IsModified(path string) bool {
	c.mu.RLock()
	e, ok := c.entries[path]
	c.mu.RUnlock()
	if !ok {
		return false
	}

	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return info.ModTime().After(e.ModTime) || info.Size() != e.Size
}

// ModifiedPaths returns all cached paths that have been modified on disk.
func (c *Cache) ModifiedPaths() []string {
	c.mu.RLock()
	paths := make([]string, 0, len(c.entries))
	for p := range c.entries {
		paths = append(paths, p)
	}
	c.mu.RUnlock()

	var modified []string
	for _, p := range paths {
		if c.IsModified(p) {
			modified = append(modified, p)
		}
	}
	return modified
}

// Size returns the number of cached entries.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *Cache) evictOldest() {
	var oldest string
	var oldestTime time.Time
	for path, e := range c.entries {
		if oldest == "" || e.CachedAt.Before(oldestTime) {
			oldest = path
			oldestTime = e.CachedAt
		}
	}
	if oldest != "" {
		delete(c.entries, oldest)
	}
}
