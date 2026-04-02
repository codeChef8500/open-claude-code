package memory

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

// SessionMemoryConfig controls when automatic memory extraction triggers.
type SessionMemoryConfig struct {
	// MinTokensToInit is the minimum context window token count before
	// session memory is initialized for the first time.
	MinTokensToInit int
	// MinTokensBetweenUpdates is the minimum token growth between extractions.
	MinTokensBetweenUpdates int
	// ToolCallsBetweenUpdates is the minimum tool calls between extractions.
	ToolCallsBetweenUpdates int
	// Enabled controls whether automatic extraction is active.
	Enabled bool
}

// DefaultSessionMemoryConfig returns sensible defaults.
func DefaultSessionMemoryConfig() SessionMemoryConfig {
	return SessionMemoryConfig{
		MinTokensToInit:         8000,
		MinTokensBetweenUpdates: 4000,
		ToolCallsBetweenUpdates: 5,
		Enabled:                 true,
	}
}

// SessionMemoryState tracks extraction state for a single session.
type SessionMemoryState struct {
	mu                   sync.Mutex
	initialized          bool
	lastExtractionTokens int
	lastExtractionTime   time.Time
	extractionInProgress atomic.Bool
	toolCallsSinceLast   int
	lastMessageID        string
	totalExtractions     int
}

// NewSessionMemoryState creates fresh tracking state.
func NewSessionMemoryState() *SessionMemoryState {
	return &SessionMemoryState{}
}

// RecordToolCall increments the tool call counter.
func (s *SessionMemoryState) RecordToolCall() {
	s.mu.Lock()
	s.toolCallsSinceLast++
	s.mu.Unlock()
}

// ShouldExtract evaluates whether extraction thresholds are met.
func (s *SessionMemoryState) ShouldExtract(cfg SessionMemoryConfig, currentTokens int) bool {
	if !cfg.Enabled {
		return false
	}
	if s.extractionInProgress.Load() {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// First extraction: require minimum tokens.
	if !s.initialized {
		if currentTokens < cfg.MinTokensToInit {
			return false
		}
		s.initialized = true
	}

	// Check token growth since last extraction.
	tokenGrowth := currentTokens - s.lastExtractionTokens
	if tokenGrowth < cfg.MinTokensBetweenUpdates {
		return false
	}

	// Check tool call threshold.
	if s.toolCallsSinceLast < cfg.ToolCallsBetweenUpdates {
		return false
	}

	return true
}

// MarkExtractionStarted sets the in-progress flag.
func (s *SessionMemoryState) MarkExtractionStarted() {
	s.extractionInProgress.Store(true)
}

// MarkExtractionCompleted records completion and resets counters.
func (s *SessionMemoryState) MarkExtractionCompleted(tokenCount int) {
	s.mu.Lock()
	s.lastExtractionTokens = tokenCount
	s.lastExtractionTime = time.Now()
	s.toolCallsSinceLast = 0
	s.totalExtractions++
	s.mu.Unlock()
	s.extractionInProgress.Store(false)
}

// Stats returns extraction statistics.
func (s *SessionMemoryState) Stats() (total int, lastTime time.Time, inProgress bool) {
	s.mu.Lock()
	total = s.totalExtractions
	lastTime = s.lastExtractionTime
	s.mu.Unlock()
	inProgress = s.extractionInProgress.Load()
	return
}

// SessionMemoryManager coordinates automatic background memory extraction.
type SessionMemoryManager struct {
	config    SessionMemoryConfig
	state     *SessionMemoryState
	store     *Store
	prov      provider.Provider
	sessionID string
}

// NewSessionMemoryManager creates a new manager.
func NewSessionMemoryManager(
	cfg SessionMemoryConfig,
	store *Store,
	prov provider.Provider,
	sessionID string,
) *SessionMemoryManager {
	return &SessionMemoryManager{
		config:    cfg,
		state:     NewSessionMemoryState(),
		store:     store,
		prov:      prov,
		sessionID: sessionID,
	}
}

// State returns the tracking state.
func (m *SessionMemoryManager) State() *SessionMemoryState { return m.state }

// Config returns the current config.
func (m *SessionMemoryManager) Config() SessionMemoryConfig { return m.config }

// SetConfig updates the configuration.
func (m *SessionMemoryManager) SetConfig(cfg SessionMemoryConfig) { m.config = cfg }

// RecordToolCall records a tool call for threshold tracking.
func (m *SessionMemoryManager) RecordToolCall() {
	m.state.RecordToolCall()
}

// MaybeExtract checks thresholds and runs extraction if needed.
// This is designed to be called as a post-sampling hook.
// It runs extraction in the background and does not block.
func (m *SessionMemoryManager) MaybeExtract(ctx context.Context, messages []*engine.Message, currentTokens int) {
	if !m.state.ShouldExtract(m.config, currentTokens) {
		return
	}

	m.state.MarkExtractionStarted()
	go func() {
		defer m.state.MarkExtractionCompleted(currentTokens)
		memories, err := ExtractMemories(ctx, m.prov, messages, m.sessionID)
		if err != nil || len(memories) == 0 {
			return
		}
		_ = m.store.SaveAll(memories)
	}()
}

// ForceExtract runs extraction immediately, blocking until complete.
func (m *SessionMemoryManager) ForceExtract(ctx context.Context, messages []*engine.Message, currentTokens int) ([]*ExtractedMemory, error) {
	m.state.MarkExtractionStarted()
	defer m.state.MarkExtractionCompleted(currentTokens)

	memories, err := ExtractMemories(ctx, m.prov, messages, m.sessionID)
	if err != nil {
		return nil, err
	}
	if len(memories) > 0 {
		_ = m.store.SaveAll(memories)
	}
	return memories, nil
}
