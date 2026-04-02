package resultstore

import (
	"sync"
	"time"
)

// Entry holds a single tool-call result keyed by tool-use ID.
type Entry struct {
	ToolUseID string
	ToolName  string
	Input     []byte
	Output    string
	IsError   bool
	CreatedAt time.Time
}

// Store is a thread-safe in-memory store for tool call results within a
// session.  Results are keyed by their tool-use ID and can be retrieved for
// hook callbacks, audit logging, and compact summarisation.
type Store struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	order   []string // insertion order for iteration
}

// New creates an empty Store.
func New() *Store {
	return &Store{entries: make(map[string]*Entry)}
}

// Put stores a tool result.  If an entry with the same ID already exists it is
// overwritten.
func (s *Store) Put(e *Entry) {
	if e == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[e.ToolUseID]; !exists {
		s.order = append(s.order, e.ToolUseID)
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	s.entries[e.ToolUseID] = e
}

// Get returns the entry for a given tool-use ID.
func (s *Store) Get(toolUseID string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[toolUseID]
	return e, ok
}

// All returns all entries in insertion order.
func (s *Store) All() []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Entry, 0, len(s.order))
	for _, id := range s.order {
		if e, ok := s.entries[id]; ok {
			out = append(out, e)
		}
	}
	return out
}

// ByTool returns all entries for a specific tool name in insertion order.
func (s *Store) ByTool(toolName string) []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Entry
	for _, id := range s.order {
		if e, ok := s.entries[id]; ok && e.ToolName == toolName {
			out = append(out, e)
		}
	}
	return out
}

// Len returns the number of stored results.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Clear removes all entries.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string]*Entry)
	s.order = nil
}

// Errors returns only the entries that represent errors.
func (s *Store) Errors() []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Entry
	for _, id := range s.order {
		if e, ok := s.entries[id]; ok && e.IsError {
			out = append(out, e)
		}
	}
	return out
}
