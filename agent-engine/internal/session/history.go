package session

import "sync"

const defaultHistoryLimit = 1000

// History keeps a ring buffer of recent user input strings for recall /
// up-arrow navigation in CLI consumers.
type History struct {
	mu      sync.Mutex
	entries []string
	limit   int
}

// NewHistory creates a History with the given capacity (0 → default 1000).
func NewHistory(limit int) *History {
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	return &History{limit: limit}
}

// Add appends an entry, evicting the oldest when the limit is reached.
func (h *History) Add(entry string) {
	if entry == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.entries) >= h.limit {
		h.entries = h.entries[1:]
	}
	h.entries = append(h.entries, entry)
}

// All returns a snapshot of all entries in insertion order (oldest first).
func (h *History) All() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.entries))
	copy(out, h.entries)
	return out
}

// Last returns the n most recent entries (oldest-first within the slice).
// If n <= 0 or n >= len, all entries are returned.
func (h *History) Last(n int) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n <= 0 || n >= len(h.entries) {
		out := make([]string, len(h.entries))
		copy(out, h.entries)
		return out
	}
	src := h.entries[len(h.entries)-n:]
	out := make([]string, n)
	copy(out, src)
	return out
}

// Len returns the current number of stored entries.
func (h *History) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.entries)
}

// Clear removes all history entries.
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = h.entries[:0]
}
