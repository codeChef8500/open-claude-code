package state

import "sync"

// Signal is a typed reactive state cell.  It holds a value and notifies
// subscribers synchronously whenever the value changes.
//
// Usage mirrors SolidJS createSignal():
//
//	sig := NewSignal("initial")
//	sig.Subscribe(func(v string) { fmt.Println("changed:", v) })
//	sig.Set("next")  // subscriber fires immediately
type Signal[T comparable] struct {
	mu          sync.RWMutex
	val         T
	subscribers []func(T)
}

// NewSignal creates a Signal initialised to val.
func NewSignal[T comparable](val T) *Signal[T] {
	return &Signal[T]{val: val}
}

// Get returns the current value.
func (s *Signal[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.val
}

// Set updates the value and fires all subscribers if it changed.
func (s *Signal[T]) Set(v T) {
	s.mu.Lock()
	if s.val == v {
		s.mu.Unlock()
		return
	}
	s.val = v
	subs := make([]func(T), len(s.subscribers))
	copy(subs, s.subscribers)
	s.mu.Unlock()

	for _, fn := range subs {
		fn(v)
	}
}

// Subscribe registers fn to be called with the new value after every Set that
// changes the value.  Returns an unsubscribe function.
func (s *Signal[T]) Subscribe(fn func(T)) func() {
	s.mu.Lock()
	s.subscribers = append(s.subscribers, fn)
	idx := len(s.subscribers) - 1
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		// Nil out the slot to avoid shifting indices.
		if idx < len(s.subscribers) {
			s.subscribers[idx] = nil
		}
	}
}

// AnySignal is a type-erased Signal for use in generic pipelines.
type AnySignal interface {
	notify()
}
