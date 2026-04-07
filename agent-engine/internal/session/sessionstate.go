package session

import (
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Rich session state — aligned with claude-code-main src/utils/sessionState.ts
// ────────────────────────────────────────────────────────────────────────────

// SessionStateKind represents the current phase of the session.
type SessionStateKind string

const (
	StateIdle          SessionStateKind = "idle"
	StateProcessing    SessionStateKind = "processing"
	StatePendingAction SessionStateKind = "pending_action"
	StateWaiting       SessionStateKind = "waiting"
	StateCompleted     SessionStateKind = "completed"
	StateError         SessionStateKind = "error"
)

// RequiresActionDetails holds info when the session is waiting for user input.
type RequiresActionDetails struct {
	ToolUseID   string `json:"tool_use_id,omitempty"`
	ToolName    string `json:"tool_name,omitempty"`
	Description string `json:"description,omitempty"`
}

// SessionExternalMetadata holds external-facing metadata about the session,
// suitable for display in UIs, dashboards, or the status bar.
type SessionExternalMetadata struct {
	Title            string                 `json:"title,omitempty"`
	State            SessionStateKind       `json:"state"`
	PendingAction    *RequiresActionDetails `json:"pending_action,omitempty"`
	TurnCount        int                    `json:"turn_count"`
	TotalInputTokens int                    `json:"total_input_tokens"`
	TotalOutputTokens int                   `json:"total_output_tokens"`
	CostUSD          float64                `json:"cost_usd"`
	Model            string                 `json:"model,omitempty"`
	CompactCount     int                    `json:"compact_count"`
	LastActivityAt   time.Time              `json:"last_activity_at"`
}

// SessionState is the live, mutable state of a running session.
// It is thread-safe and supports listener notifications.
type SessionState struct {
	mu        sync.RWMutex
	state     SessionStateKind
	metadata  SessionExternalMetadata
	listeners []func(SessionExternalMetadata)
}

// NewSessionState creates a new session state in idle.
func NewSessionState() *SessionState {
	return &SessionState{
		state: StateIdle,
		metadata: SessionExternalMetadata{
			State: StateIdle,
		},
	}
}

// Get returns the current state kind.
func (s *SessionState) Get() SessionStateKind {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetMetadata returns a snapshot of the external metadata.
func (s *SessionState) GetMetadata() SessionExternalMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metadata
}

// SetState transitions the session state and notifies listeners.
func (s *SessionState) SetState(kind SessionStateKind) {
	s.mu.Lock()
	s.state = kind
	s.metadata.State = kind
	s.metadata.LastActivityAt = time.Now()
	meta := s.metadata
	listeners := s.listeners
	s.mu.Unlock()
	for _, fn := range listeners {
		fn(meta)
	}
}

// SetPendingAction sets the pending action details.
func (s *SessionState) SetPendingAction(details *RequiresActionDetails) {
	s.mu.Lock()
	s.metadata.PendingAction = details
	if details != nil {
		s.state = StatePendingAction
		s.metadata.State = StatePendingAction
	}
	meta := s.metadata
	listeners := s.listeners
	s.mu.Unlock()
	for _, fn := range listeners {
		fn(meta)
	}
}

// ClearPendingAction clears the pending action and returns to processing.
func (s *SessionState) ClearPendingAction() {
	s.mu.Lock()
	s.metadata.PendingAction = nil
	if s.state == StatePendingAction {
		s.state = StateProcessing
		s.metadata.State = StateProcessing
	}
	meta := s.metadata
	listeners := s.listeners
	s.mu.Unlock()
	for _, fn := range listeners {
		fn(meta)
	}
}

// RecordTurn increments the turn counter.
func (s *SessionState) RecordTurn() {
	s.mu.Lock()
	s.metadata.TurnCount++
	s.metadata.LastActivityAt = time.Now()
	s.mu.Unlock()
}

// RecordTokenUsage adds token usage.
func (s *SessionState) RecordTokenUsage(inputTokens, outputTokens int) {
	s.mu.Lock()
	s.metadata.TotalInputTokens += inputTokens
	s.metadata.TotalOutputTokens += outputTokens
	s.mu.Unlock()
}

// RecordCost adds cost in USD.
func (s *SessionState) RecordCost(costUSD float64) {
	s.mu.Lock()
	s.metadata.CostUSD += costUSD
	s.mu.Unlock()
}

// RecordCompaction increments the compaction counter.
func (s *SessionState) RecordCompaction() {
	s.mu.Lock()
	s.metadata.CompactCount++
	s.mu.Unlock()
}

// SetTitle sets the session title.
func (s *SessionState) SetTitle(title string) {
	s.mu.Lock()
	s.metadata.Title = title
	meta := s.metadata
	listeners := s.listeners
	s.mu.Unlock()
	for _, fn := range listeners {
		fn(meta)
	}
}

// SetModel sets the model name.
func (s *SessionState) SetModel(model string) {
	s.mu.Lock()
	s.metadata.Model = model
	s.mu.Unlock()
}

// OnChange registers a listener called whenever metadata changes.
func (s *SessionState) OnChange(fn func(SessionExternalMetadata)) {
	s.mu.Lock()
	s.listeners = append(s.listeners, fn)
	s.mu.Unlock()
}
