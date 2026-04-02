package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store provides persistent CRUD operations for extracted memories.
// Memories are stored as JSON files in a configurable directory.
type Store struct {
	mu      sync.RWMutex
	dir     string
	cache   map[string]*ExtractedMemory // id -> memory
	loaded  bool
}

// NewStore creates a Store that persists memories under dir.
func NewStore(dir string) *Store {
	return &Store{
		dir:   dir,
		cache: make(map[string]*ExtractedMemory),
	}
}

// Dir returns the storage directory.
func (s *Store) Dir() string { return s.dir }

// ensureDir creates the storage directory if it doesn't exist.
func (s *Store) ensureDir() error {
	return os.MkdirAll(s.dir, 0700)
}

// Load reads all memory files from disk into the in-memory cache.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDir(); err != nil {
		return fmt.Errorf("memory store: mkdir: %w", err)
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("memory store: readdir: %w", err)
	}

	s.cache = make(map[string]*ExtractedMemory)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var mem ExtractedMemory
		if err := json.Unmarshal(data, &mem); err != nil {
			continue
		}
		if mem.ID != "" {
			s.cache[mem.ID] = &mem
		}
	}
	s.loaded = true
	return nil
}

// Save persists a single memory to disk.
func (s *Store) Save(mem *ExtractedMemory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(mem, "", "  ")
	if err != nil {
		return fmt.Errorf("memory store: marshal: %w", err)
	}

	path := filepath.Join(s.dir, mem.ID+".json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("memory store: write: %w", err)
	}

	s.cache[mem.ID] = mem
	return nil
}

// SaveAll persists multiple memories.
func (s *Store) SaveAll(memories []*ExtractedMemory) error {
	for _, m := range memories {
		if err := s.Save(m); err != nil {
			return err
		}
	}
	return nil
}

// Get retrieves a memory by ID.
func (s *Store) Get(id string) *ExtractedMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[id]
}

// Delete removes a memory by ID from disk and cache.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.cache, id)
	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("memory store: delete: %w", err)
	}
	return nil
}

// All returns all cached memories sorted by extraction time (newest first).
func (s *Store) All() []*ExtractedMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ExtractedMemory, 0, len(s.cache))
	for _, m := range s.cache {
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ExtractedAt.After(result[j].ExtractedAt)
	})
	return result
}

// BySession returns memories for a specific session.
func (s *Store) BySession(sessionID string) []*ExtractedMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ExtractedMemory
	for _, m := range s.cache {
		if m.SessionID == sessionID {
			result = append(result, m)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ExtractedAt.After(result[j].ExtractedAt)
	})
	return result
}

// ByType returns memories matching the given type tag.
func (s *Store) ByType(memType string) []*ExtractedMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ExtractedMemory
	for _, m := range s.cache {
		for _, t := range m.Tags {
			if t == memType {
				result = append(result, m)
				break
			}
		}
	}
	return result
}

// Count returns the total number of stored memories.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cache)
}

// Clear removes all memories from disk and cache.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id := range s.cache {
		path := filepath.Join(s.dir, id+".json")
		_ = os.Remove(path)
	}
	s.cache = make(map[string]*ExtractedMemory)
	return nil
}

// ClearOlderThan removes memories older than the given duration.
func (s *Store) ClearOlderThan(age time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-age)
	var removed int
	for id, m := range s.cache {
		if m.ExtractedAt.Before(cutoff) {
			path := filepath.Join(s.dir, id+".json")
			_ = os.Remove(path)
			delete(s.cache, id)
			removed++
		}
	}
	return removed, nil
}

// Deduplicate removes memories with identical content, keeping the newest.
func (s *Store) Deduplicate() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]*ExtractedMemory)
	var toDelete []string

	for id, m := range s.cache {
		if existing, ok := seen[m.Content]; ok {
			// Keep the newer one.
			if m.ExtractedAt.After(existing.ExtractedAt) {
				toDelete = append(toDelete, existing.ID)
				seen[m.Content] = m
			} else {
				toDelete = append(toDelete, id)
			}
		} else {
			seen[m.Content] = m
		}
	}

	for _, id := range toDelete {
		delete(s.cache, id)
		path := filepath.Join(s.dir, id+".json")
		_ = os.Remove(path)
	}

	return len(toDelete)
}
