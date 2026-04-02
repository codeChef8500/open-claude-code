package engine

import "sync"

// TokenWarningLevel classifies how close to the context limit we are.
type TokenWarningLevel int

const (
	TokenWarningNone     TokenWarningLevel = 0
	TokenWarningApproach TokenWarningLevel = 1 // ~80 % used
	TokenWarningHigh     TokenWarningLevel = 2 // ~90 % used
	TokenWarningBlocking TokenWarningLevel = 3 // ≥ 95 % — must compact
)

// tokenApproachFraction is the threshold for TokenWarningApproach.
const (
	tokenApproachFraction = 0.80
	tokenHighFraction     = 0.90
	tokenBlockingFraction = 0.95
)

// TokenBudgetTracker tracks cumulative token usage across compaction
// boundaries and exposes helpers used by the query loop.
type TokenBudgetTracker struct {
	mu sync.Mutex

	contextWindowSize int
	inputTokens       int
	outputTokens      int
	cacheReadTokens   int
	cacheWriteTokens  int

	// compactedAt is the input-token count at the last successful compaction,
	// used to detect if usage has grown since the last compact.
	compactedAt int
}

// NewTokenBudgetTracker creates a tracker for a given context window size.
func NewTokenBudgetTracker(contextWindowSize int) *TokenBudgetTracker {
	if contextWindowSize <= 0 {
		contextWindowSize = 200_000 // sane default for claude-3.x
	}
	return &TokenBudgetTracker{contextWindowSize: contextWindowSize}
}

// Update records the latest usage figures from an API response.
func (t *TokenBudgetTracker) Update(inputTokens, outputTokens, cacheRead, cacheWrite int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inputTokens = inputTokens
	t.outputTokens = outputTokens
	t.cacheReadTokens = cacheRead
	t.cacheWriteTokens = cacheWrite
}

// UsedFraction returns the fraction of the context window occupied by input
// tokens (0.0–1.0).
func (t *TokenBudgetTracker) UsedFraction() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.contextWindowSize == 0 {
		return 0
	}
	return float64(t.inputTokens) / float64(t.contextWindowSize)
}

// WarningLevel returns the current warning severity.
func (t *TokenBudgetTracker) WarningLevel() TokenWarningLevel {
	f := t.UsedFraction()
	switch {
	case f >= tokenBlockingFraction:
		return TokenWarningBlocking
	case f >= tokenHighFraction:
		return TokenWarningHigh
	case f >= tokenApproachFraction:
		return TokenWarningApproach
	default:
		return TokenWarningNone
	}
}

// IsAtBlockingLimit reports whether the context is too full to continue
// without compaction.
func (t *TokenBudgetTracker) IsAtBlockingLimit() bool {
	return t.WarningLevel() == TokenWarningBlocking
}

// MarkCompacted records the current input-token count as the compaction
// baseline.
func (t *TokenBudgetTracker) MarkCompacted() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.compactedAt = t.inputTokens
}

// GrownSinceCompact returns true if input tokens have grown by more than
// the given delta since the last compaction.
func (t *TokenBudgetTracker) GrownSinceCompact(delta int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.inputTokens-t.compactedAt > delta
}

// Snapshot returns a read-only copy of the current usage.
func (t *TokenBudgetTracker) Snapshot() TokenBudgetState {
	t.mu.Lock()
	defer t.mu.Unlock()
	frac := float64(0)
	if t.contextWindowSize > 0 {
		frac = float64(t.inputTokens) / float64(t.contextWindowSize)
	}
	_ = frac // UsageFraction() is a method on TokenBudgetState; not a stored field
	return TokenBudgetState{
		InputTokens:       t.inputTokens,
		OutputTokens:      t.outputTokens,
		CacheReadTokens:   t.cacheReadTokens,
		CacheWriteTokens:  t.cacheWriteTokens,
		ContextWindowSize: t.contextWindowSize,
	}
}

// CalculateTokenWarningState is a pure function (no receiver) for computing
// the warning level from raw counts — usable in tests and compact pipeline.
func CalculateTokenWarningState(inputTokens, contextWindowSize int) TokenWarningLevel {
	if contextWindowSize <= 0 {
		return TokenWarningNone
	}
	f := float64(inputTokens) / float64(contextWindowSize)
	switch {
	case f >= tokenBlockingFraction:
		return TokenWarningBlocking
	case f >= tokenHighFraction:
		return TokenWarningHigh
	case f >= tokenApproachFraction:
		return TokenWarningApproach
	default:
		return TokenWarningNone
	}
}
