package engine

import (
	"log/slog"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Transcript & Session persistence — records the full conversation and
// manages session lifecycle.
// Aligned with claude-code-main SessionManager and Transcript types.
// ────────────────────────────────────────────────────────────────────────────

// TranscriptEntry is a single entry in the conversation transcript.
// It augments Message with timing and metadata.
type TranscriptEntry struct {
	// Message is the conversation message.
	Message *Message
	// Timestamp is when the entry was recorded.
	Timestamp time.Time
	// TurnIndex is the turn number within the session.
	TurnIndex int
	// Source indicates the origin (user, assistant, system, tool).
	Source string
	// DurationMs is the time to produce this entry (for assistant messages).
	DurationMs int64
	// TokensUsed is the tokens consumed for this entry (for assistant messages).
	TokensUsed int
}

// Transcript accumulates conversation entries for a single session.
// Thread-safe.
type Transcript struct {
	mu      sync.RWMutex
	entries []TranscriptEntry
	// sessionID is the session this transcript belongs to.
	sessionID string
	// startedAt is when the session started.
	startedAt time.Time
}

// NewTranscript creates a transcript for a session.
func NewTranscript(sessionID string) *Transcript {
	return &Transcript{
		sessionID: sessionID,
		startedAt: time.Now(),
	}
}

// Append adds a message to the transcript.
func (t *Transcript) Append(msg *Message, source string, durationMs int64, tokensUsed int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, TranscriptEntry{
		Message:    msg,
		Timestamp:  time.Now(),
		TurnIndex:  len(t.entries),
		Source:     source,
		DurationMs: durationMs,
		TokensUsed: tokensUsed,
	})
}

// Entries returns a snapshot of all entries.
func (t *Transcript) Entries() []TranscriptEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]TranscriptEntry, len(t.entries))
	copy(out, t.entries)
	return out
}

// Len returns the number of entries.
func (t *Transcript) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.entries)
}

// SessionID returns the session ID.
func (t *Transcript) SessionID() string { return t.sessionID }

// Duration returns the elapsed time since the session started.
func (t *Transcript) Duration() time.Duration { return time.Since(t.startedAt) }

// ── Session persistence coordinator ──────────────────────────────────────

// SessionPersister coordinates writing transcript entries to durable storage
// via the SessionWriter interface.
type SessionPersister struct {
	writer    SessionWriter
	sessionID string
	mu        sync.Mutex
	// pendingCount tracks how many writes are in-flight.
	pendingCount int
}

// NewSessionPersister creates a persister for a session.
func NewSessionPersister(writer SessionWriter, sessionID string) *SessionPersister {
	return &SessionPersister{
		writer:    writer,
		sessionID: sessionID,
	}
}

// PersistMessage writes a message to durable storage.
// It is goroutine-safe and non-blocking (logs errors instead of returning them).
func (p *SessionPersister) PersistMessage(msg *Message) {
	if p.writer == nil || msg == nil {
		return
	}
	p.mu.Lock()
	p.pendingCount++
	p.mu.Unlock()

	go func() {
		defer func() {
			p.mu.Lock()
			p.pendingCount--
			p.mu.Unlock()
		}()
		if err := p.writer.AppendMessage(p.sessionID, msg); err != nil {
			slog.Warn("session_persister: write failed",
				slog.String("session_id", p.sessionID),
				slog.String("msg_uuid", msg.UUID),
				slog.Any("err", err))
		}
	}()
}

// PendingWrites returns the number of in-flight writes.
func (p *SessionPersister) PendingWrites() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pendingCount
}

// ── Session state management ─────────────────────────────────────────────

// SessionState tracks the lifecycle of a conversation session.
type SessionState struct {
	mu sync.RWMutex

	// ID is the session identifier.
	ID string
	// StartedAt is when the session was created.
	StartedAt time.Time
	// LastActivityAt is the timestamp of the last user or assistant message.
	LastActivityAt time.Time
	// TurnCount is the total number of turns completed.
	TurnCount int
	// QueryCount is the total number of queries submitted.
	QueryCount int
	// TotalCostUSD is the cumulative cost.
	TotalCostUSD float64
	// TotalInputTokens is the cumulative input tokens.
	TotalInputTokens int
	// TotalOutputTokens is the cumulative output tokens.
	TotalOutputTokens int
	// IsActive is true while a query is running.
	IsActive bool
	// LastModel is the model used in the last query.
	LastModel string
}

// NewSessionState creates a new session state.
func NewSessionState(id string) *SessionState {
	now := time.Now()
	return &SessionState{
		ID:             id,
		StartedAt:      now,
		LastActivityAt: now,
	}
}

// RecordQueryStart marks the start of a new query.
func (s *SessionState) RecordQueryStart(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.QueryCount++
	s.IsActive = true
	s.LastModel = model
	s.LastActivityAt = time.Now()
}

// RecordQueryEnd marks the end of a query with usage stats.
func (s *SessionState) RecordQueryEnd(usage *UsageStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IsActive = false
	s.LastActivityAt = time.Now()
	if usage != nil {
		s.TotalCostUSD += usage.CostUSD
		s.TotalInputTokens += usage.InputTokens
		s.TotalOutputTokens += usage.OutputTokens
	}
}

// RecordTurn increments the turn counter.
func (s *SessionState) RecordTurn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TurnCount++
}

// Snapshot returns a read-only copy.
func (s *SessionState) Snapshot() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SessionState{
		ID:                s.ID,
		StartedAt:         s.StartedAt,
		LastActivityAt:    s.LastActivityAt,
		TurnCount:         s.TurnCount,
		QueryCount:        s.QueryCount,
		TotalCostUSD:      s.TotalCostUSD,
		TotalInputTokens:  s.TotalInputTokens,
		TotalOutputTokens: s.TotalOutputTokens,
		IsActive:          s.IsActive,
		LastModel:         s.LastModel,
	}
}
