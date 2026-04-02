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

	// PermissionMode is "default", "plan", "bypassPermissions", or "auto".
	PermissionMode string

	// PlanModeActive is true when the session is in plan-only mode.
	PlanModeActive bool

	// MCPConnected is true when at least one MCP server is connected.
	MCPConnected bool

	// ActiveMCPServers is the list of connected MCP server names.
	ActiveMCPServers []string

	// TokenBudget tracks the current context window usage fraction (0–1).
	TokenBudgetFraction float64

	// InputTokens is the most recent cumulative input token count.
	InputTokens int

	// OutputTokens is the most recent cumulative output token count.
	OutputTokens int

	// ActiveToolName is the tool currently being executed (empty if idle).
	ActiveToolName string

	// IsStreaming is true while the engine is streaming a response.
	IsStreaming bool

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
