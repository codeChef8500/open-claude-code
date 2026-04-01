package util

import "sync"

// Signal is a generic reactive value that notifies registered listeners
// whenever its value changes. It is the Go equivalent of the TypeScript
// createSignal() reactive primitive.
type Signal[T any] struct {
	mu        sync.RWMutex
	value     T
	listeners []func(T)
}

// NewSignal creates a Signal initialised to initialValue.
func NewSignal[T any](initialValue T) *Signal[T] {
	return &Signal[T]{value: initialValue}
}

// Get returns the current value of the signal.
func (s *Signal[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

// Set updates the signal value and notifies all registered listeners.
func (s *Signal[T]) Set(newValue T) {
	s.mu.Lock()
	s.value = newValue
	listeners := make([]func(T), len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	for _, l := range listeners {
		l(newValue)
	}
}

// Subscribe registers a callback that is invoked after each Set call.
// Returns an unsubscribe function.
func (s *Signal[T]) Subscribe(fn func(T)) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)

	// Return a cancel function that removes this listener.
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, l := range s.listeners {
			// Compare by pointer — closures allocated at different call sites are distinct.
			if &l == &fn {
				s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
				return
			}
		}
	}
}

// Update applies fn to the current value atomically and notifies listeners.
func (s *Signal[T]) Update(fn func(T) T) {
	s.mu.Lock()
	newVal := fn(s.value)
	s.value = newVal
	listeners := make([]func(T), len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	for _, l := range listeners {
		l(newVal)
	}
}
