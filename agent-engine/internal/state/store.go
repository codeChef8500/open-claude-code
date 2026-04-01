package state

import (
	"sync"
	"sync/atomic"
)

// Store is a thread-safe key-value store that notifies subscribers on change.
// It is used as the central mutable state for the agent engine.
type Store struct {
	mu        sync.RWMutex
	data      map[string]interface{}
	listeners []func(key string, val interface{})

	// Cost accumulator uses atomic for lock-free increments.
	totalCostUSD atomic.Int64 // stored as micro-USD (multiply by 1e-6)
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{data: make(map[string]interface{})}
}

// Set stores value under key and notifies listeners.
func (s *Store) Set(key string, val interface{}) {
	s.mu.Lock()
	s.data[key] = val
	listeners := make([]func(string, interface{}), len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	for _, l := range listeners {
		l(key, val)
	}
}

// Get retrieves the value stored under key. Returns nil if not found.
func (s *Store) Get(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// GetString is a typed convenience accessor for string values.
func (s *Store) GetString(key string) string {
	v := s.Get(key)
	if str, ok := v.(string); ok {
		return str
	}
	return ""
}

// GetBool is a typed convenience accessor for bool values.
func (s *Store) GetBool(key string) bool {
	v := s.Get(key)
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// Delete removes key from the store and notifies listeners.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	listeners := make([]func(string, interface{}), len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	for _, l := range listeners {
		l(key, nil)
	}
}

// Subscribe registers a listener called after every Set or Delete.
func (s *Store) Subscribe(fn func(key string, val interface{})) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}

// AddCostUSD atomically accumulates USD cost (precision: 1 micro-dollar).
func (s *Store) AddCostUSD(usd float64) {
	microUSD := int64(usd * 1_000_000)
	s.totalCostUSD.Add(microUSD)
}

// TotalCostUSD returns the accumulated cost in USD.
func (s *Store) TotalCostUSD() float64 {
	return float64(s.totalCostUSD.Load()) / 1_000_000
}
