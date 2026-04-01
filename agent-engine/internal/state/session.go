package state

import (
	"sync"
	"sync/atomic"
)

// SessionState tracks the mutable state of a single engine session.
// It is created per Engine.SubmitMessage call chain and is goroutine-safe.
type SessionState struct {
	mu sync.RWMutex

	sessionID string
	workDir   string

	// Token and cost counters (atomic for hot-path reads)
	totalInputTokens  atomic.Int64
	totalOutputTokens atomic.Int64
	totalCostMicroUSD atomic.Int64 // stored as micro-dollars

	// Turn counter
	turnCount atomic.Int32

	// Whether the context has been compacted at least once.
	compacted bool
}

// NewSessionState initialises a new session state.
func NewSessionState(sessionID, workDir string) *SessionState {
	return &SessionState{
		sessionID: sessionID,
		workDir:   workDir,
	}
}

// SessionID returns the session identifier.
func (s *SessionState) SessionID() string { return s.sessionID }

// WorkDir returns the working directory for this session.
func (s *SessionState) WorkDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workDir
}

// SetWorkDir updates the working directory.
func (s *SessionState) SetWorkDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workDir = dir
}

// AddUsage accumulates token counts and cost from a single LLM response.
func (s *SessionState) AddUsage(inputTokens, outputTokens int, costUSD float64) {
	s.totalInputTokens.Add(int64(inputTokens))
	s.totalOutputTokens.Add(int64(outputTokens))
	s.totalCostMicroUSD.Add(int64(costUSD * 1_000_000))
}

// TotalCostUSD returns the accumulated cost for this session.
func (s *SessionState) TotalCostUSD() float64 {
	return float64(s.totalCostMicroUSD.Load()) / 1_000_000
}

// TotalTokens returns cumulative input+output token count.
func (s *SessionState) TotalTokens() int {
	return int(s.totalInputTokens.Load() + s.totalOutputTokens.Load())
}

// IncrTurn increments the turn counter and returns the new value.
func (s *SessionState) IncrTurn() int {
	return int(s.turnCount.Add(1))
}

// TurnCount returns the current turn count.
func (s *SessionState) TurnCount() int {
	return int(s.turnCount.Load())
}

// SetCompacted marks that the context has been compacted.
func (s *SessionState) SetCompacted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compacted = true
}

// IsCompacted reports whether context compaction has occurred.
func (s *SessionState) IsCompacted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.compacted
}
