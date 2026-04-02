package engine

import "time"

// QueryConfig holds per-query tuning knobs that callers can override for a
// single SubmitMessage call.  Zero-value fields fall back to the engine-level
// EngineConfig defaults.
type QueryConfig struct {
	// MaxTokens overrides EngineConfig.MaxTokens for this query.
	MaxTokens int
	// ThinkingBudget is the extended-thinking token budget (0 = disabled).
	ThinkingBudget int
	// Temperature overrides the model temperature (nil = use default).
	Temperature *float64
	// Model overrides the model name for this query only.
	Model string
	// Timeout is the maximum wall-clock time allowed for the full query loop.
	// 0 means no per-query deadline (only the ctx deadline applies).
	Timeout time.Duration
	// DisableCompaction skips auto-compaction for this query even if the
	// context window is near its limit.
	DisableCompaction bool
	// ToolFilter, if non-nil, restricts which tool names the model may call
	// in this query.  An empty slice means "no tools".  nil means all tools.
	ToolFilter []string
	// PlanMode forces the session into plan-only mode for this query.
	PlanMode bool
}

// QueryDeps bundles the optional per-query dependency overrides that the
// caller can inject to customise engine behaviour without reconfiguring the
// whole engine.
type QueryDeps struct {
	// MemoryLoader overrides the engine-level MemoryLoader for this query.
	MemoryLoader MemoryLoader
	// SystemPromptBuilder overrides the engine-level SystemPromptBuilder.
	SystemPromptBuilder SystemPromptBuilder
	// GlobalPermissionChecker overrides the engine-level checker.
	GlobalPermissionChecker GlobalPermissionChecker
	// AutoModeClassifier overrides the engine-level classifier.
	AutoModeClassifier AutoModeClassifier
	// ExtraSystemPrompt is appended to the effective system prompt.
	ExtraSystemPrompt string
	// ExtraTools are registered for this query only (not persisted to the
	// engine registry).
	ExtraTools []Tool
}

// TokenBudgetState tracks live token usage within a single query loop
// iteration.  It is updated after every model response and is used to decide
// when to compact, warn, or stop.
type TokenBudgetState struct {
	// InputTokens is the number of input tokens consumed so far.
	InputTokens int
	// OutputTokens is the number of output (completion) tokens used.
	OutputTokens int
	// CacheReadTokens is the number of tokens served from the prompt cache.
	CacheReadTokens int
	// CacheWriteTokens is the number of tokens written to the prompt cache.
	CacheWriteTokens int
	// ContextWindowSize is the model's maximum context window (from config).
	ContextWindowSize int
	// CompactionThreshold is the fraction of ContextWindowSize at which
	// auto-compaction is triggered (e.g. 0.85).
	CompactionThreshold float64
}

// UsageFraction returns the fraction of the context window currently consumed
// by input tokens.
func (t *TokenBudgetState) UsageFraction() float64 {
	if t.ContextWindowSize <= 0 {
		return 0
	}
	return float64(t.InputTokens) / float64(t.ContextWindowSize)
}

// ShouldCompact reports whether the context window is full enough to trigger
// auto-compaction.
func (t *TokenBudgetState) ShouldCompact() bool {
	threshold := t.CompactionThreshold
	if threshold <= 0 {
		threshold = 0.85
	}
	return t.UsageFraction() >= threshold
}

// WarningFraction is the usage fraction at which a soft warning is emitted.
const WarningFraction = 0.75
