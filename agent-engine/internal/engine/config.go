package engine

import (
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/util"
)

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
	// FallbackModel is the model to switch to on FallbackTriggeredError.
	FallbackModel string
	// MaxTurns caps the number of tool-use turns. 0 = use default (100).
	MaxTurns int
	// MaxBudgetUSD caps the total USD cost for this query.
	MaxBudgetUSD *float64
}

// ────────────────────────────────────────────────────────────────────────────
// QueryLoopConfig — immutable values snapshotted once at query() entry.
// Aligned with claude-code-main query/config.ts QueryConfig.
// Separating these from per-iteration loopState and mutable ToolUseContext
// makes a future pure step() reducer tractable.
// ────────────────────────────────────────────────────────────────────────────

// QueryLoopConfig is the immutable configuration snapshot taken at the start
// of each queryLoop invocation.  It mirrors the TS QueryConfig from
// query/config.ts.
type QueryLoopConfig struct {
	// SessionID is the session identifier snapshotted at query() entry.
	SessionID string

	// Gates are runtime flags snapshotted once per query invocation.
	// NOT feature() gates — those stay inline at guarded blocks.
	Gates QueryGates
}

// QueryGates holds runtime gate values snapshotted at query() entry.
// Aligned with the TS gates object in query/config.ts.
type QueryGates struct {
	// StreamingToolExecution enables streaming tool execution.
	StreamingToolExecution bool
	// EmitToolUseSummaries enables tool use summary generation.
	EmitToolUseSummaries bool
	// IsAnt is true for Anthropic employees (enables extra logging).
	IsAnt bool
	// FastModeEnabled is true unless explicitly disabled.
	FastModeEnabled bool
}

// BuildQueryLoopConfig snapshots immutable environment state for a queryLoop
// invocation. Aligned with claude-code-main query/config.ts buildQueryConfig.
func BuildQueryLoopConfig(sessionID string, flags *util.FeatureFlagStore) QueryLoopConfig {
	if sessionID == "" {
		sessionID = uuid.New().String()
	}
	cfg := QueryLoopConfig{
		SessionID: sessionID,
		Gates: QueryGates{
			FastModeEnabled: true, // default on
		},
	}

	// Snapshot feature flags.
	if flags != nil {
		cfg.Gates.StreamingToolExecution = flags.IsEnabled(util.FlagStreamingToolExec)
	}

	// Env overrides.
	if isEnvTruthy(os.Getenv("AGENT_ENGINE_EMIT_TOOL_USE_SUMMARIES")) {
		cfg.Gates.EmitToolUseSummaries = true
	}
	if os.Getenv("USER_TYPE") == "ant" {
		cfg.Gates.IsAnt = true
	}
	if isEnvTruthy(os.Getenv("AGENT_ENGINE_DISABLE_FAST_MODE")) {
		cfg.Gates.FastModeEnabled = false
	}

	return cfg
}

// isEnvTruthy returns true if the value is "1", "true", "yes", or "on".
func isEnvTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
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

	// ── DI function overrides (aligned with query/deps.ts) ──────────────

	// CallModelOverride replaces the engine-level ModelCaller for this query.
	// If nil, the engine's default caller is used.
	CallModelOverride ModelCaller

	// MicrocompactOverride replaces the default micro-compaction function.
	// Signature: (messages, config) -> (messages, info, error)
	MicrocompactOverride func(msgs []*Message, cfg MicrocompactConfig) ([]*Message, *MicrocompactInfo, error)

	// AutocompactOverride replaces the default auto-compaction function.
	// Signature: (ctx, messages, caller, model, config) -> (*CompactionResult, error)
	AutocompactOverride func(ctx interface{}, msgs []*Message, caller ModelCaller, model string, cfg AutocompactConfig) (*CompactionResult, error)

	// UUIDOverride replaces the default UUID generator.
	UUIDOverride func() string
}

// MicrocompactConfig holds configuration for micro-compaction.
// Aligned with claude-code-main services/compact/microCompact.ts.
type MicrocompactConfig struct {
	// ProtectLastN is the number of recent messages protected from compaction.
	ProtectLastN int
	// MaxToolResultChars is the max chars for tool result truncation.
	MaxToolResultChars int
}

// MicrocompactInfo holds the results of a micro-compaction pass.
type MicrocompactInfo struct {
	// TokensFreed is the estimated tokens reclaimed.
	TokensFreed int
	// EntriesRemoved is the number of entries removed or truncated.
	EntriesRemoved int
}

// AutocompactConfig holds configuration for auto-compaction.
type AutocompactConfig struct {
	// Model is the model to use for summarisation.
	Model string
	// MaxOutputTokens caps the summary output.
	MaxOutputTokens int
	// CustomInstructions are injected into the compact prompt.
	CustomInstructions string
}

// CompactionResult holds the output of an auto-compaction pass.
// Aligned with claude-code-main services/compact/autoCompact.ts.
type CompactionResult struct {
	// SummaryMessages are the replacement messages after compaction.
	SummaryMessages []*Message
	// MessagesToKeep are the tail messages preserved from the original.
	MessagesToKeep []*Message
	// PreCompactTokenCount is the estimated token count before compaction.
	PreCompactTokenCount int
	// PostCompactTokenCount is the estimated token count after compaction.
	PostCompactTokenCount int
	// TruePostCompactTokenCount is the API-reported count (if available).
	TruePostCompactTokenCount int
	// CompactionUsage is the token usage of the compaction call itself.
	CompactionUsage *UsageStats
	// UserDisplayMessage is a human-readable summary of what happened.
	UserDisplayMessage string
}

// EffectiveCaller returns the model caller to use for this query,
// preferring the per-query override if set.
func (d *QueryDeps) EffectiveCaller(fallback ModelCaller) ModelCaller {
	if d.CallModelOverride != nil {
		return d.CallModelOverride
	}
	return fallback
}

// EffectiveUUID returns the UUID generator to use, defaulting to uuid.New().String().
func (d *QueryDeps) EffectiveUUID() func() string {
	if d.UUIDOverride != nil {
		return d.UUIDOverride
	}
	return func() string { return uuid.New().String() }
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
