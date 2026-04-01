package engine

import "context"

// ModelCaller is the interface that all LLM provider adapters must satisfy.
// Placing it here (rather than in the provider package) breaks the
// engine ↔ provider import cycle.
type ModelCaller interface {
	// Name returns a human-readable backend identifier (e.g. "anthropic").
	Name() string
	// CallModel streams a completion. The returned channel is closed when the
	// response is complete or an error occurs (signalled via EventError).
	CallModel(ctx context.Context, params CallParams) (<-chan *StreamEvent, error)
}

// SystemPromptPart is one cache-aware segment of the system prompt.
// Providers that support prompt caching (e.g. Anthropic) inject each part as a
// separate text block so stable layers benefit from cache hits independently.
type SystemPromptPart struct {
	Content    string
	CacheHint  bool // if true, attach cache_control=ephemeral to this block
}

// CallParams holds all parameters needed for a single model API call.
type CallParams struct {
	Model          string
	MaxTokens      int
	ThinkingBudget int
	Temperature    float64
	// SystemPrompt is the single-string fallback (used when SystemPromptParts is empty).
	SystemPrompt string
	// SystemPromptParts holds ordered segments for multi-block cache-aware injection.
	// When non-empty, providers should prefer these over SystemPrompt.
	SystemPromptParts []SystemPromptPart
	Messages          []*Message
	Tools             []ToolDefinition
	UsePromptCache    bool
	SkipCacheWrite    bool
	ExtraHeaders      map[string]string
}

// ToolDefinition is the wire format for a tool spec sent to the LLM.
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema interface{}
}
