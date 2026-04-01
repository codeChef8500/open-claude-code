package state

import "sync"

// AppState is the global mutable application state.
type AppState struct {
	mu sync.RWMutex

	// Working directory (may be overridden per-agent).
	CWD string

	// Current session identifier.
	SessionID string

	// Cumulative cost in USD across all sessions in this process.
	TotalCostUSD float64

	// Current model in use.
	CurrentModel string

	// Verbose / debug mode.
	Verbose bool

	// AutoMode enabled.
	AutoMode bool

	// Listeners notified on any state mutation.
	listeners []func()
}

// New creates a fresh AppState with sane defaults.
func New(cwd string) *AppState {
	return &AppState{CWD: cwd}
}

// Get returns a snapshot copy (safe for reading outside a lock).
func (s *AppState) Get() AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return AppState{
		CWD:          s.CWD,
		SessionID:    s.SessionID,
		TotalCostUSD: s.TotalCostUSD,
		CurrentModel: s.CurrentModel,
		Verbose:      s.Verbose,
		AutoMode:     s.AutoMode,
	}
}

// Update applies fn under a write lock and notifies all listeners.
func (s *AppState) Update(fn func(st *AppState)) {
	s.mu.Lock()
	fn(s)
	listeners := make([]func(), len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	for _, l := range listeners {
		l()
	}
}

// Subscribe registers a callback invoked after every Update.
func (s *AppState) Subscribe(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}
