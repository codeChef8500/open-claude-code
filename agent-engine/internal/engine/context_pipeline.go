package engine

import (
	"context"
	"log/slog"
)

// ContextPipelineConfig holds thresholds for the engine-level compression pipeline.
type ContextPipelineConfig struct {
	// AutoCompactFraction triggers LLM compaction when used/max > fraction.
	AutoCompactFraction float64
	// MaxTokens is the context window ceiling for this session.
	MaxTokens int
	// KeepLast is the number of recent messages always preserved from snipping.
	KeepLast int
}

// DefaultContextPipelineConfig returns sensible defaults.
func DefaultContextPipelineConfig() ContextPipelineConfig {
	return ContextPipelineConfig{
		AutoCompactFraction: 0.60,
		MaxTokens:           200_000,
		KeepLast:            10,
	}
}

// ContextPipeline manages the full compaction lifecycle for a session.
// It applies lightweight local passes first, falling back to LLM summarisation
// when the context window fills beyond AutoCompactFraction.
type ContextPipeline struct {
	cfg      ContextPipelineConfig
	provider ModelCaller
	model    string
}

// NewContextPipeline creates a pipeline backed by the given LLM provider.
func NewContextPipeline(provider ModelCaller, model string, cfg ContextPipelineConfig) *ContextPipeline {
	return &ContextPipeline{cfg: cfg, provider: provider, model: model}
}

// MaybeCompact runs the pipeline when token usage exceeds the auto-compact
// threshold.  Returns the (possibly unchanged) message slice and a flag
// indicating whether compaction was performed.
func (p *ContextPipeline) MaybeCompact(ctx context.Context, messages []*Message, usedTokens int) ([]*Message, bool, error) {
	if p.cfg.MaxTokens <= 0 {
		return messages, false, nil
	}
	fraction := float64(usedTokens) / float64(p.cfg.MaxTokens)
	if fraction < p.cfg.AutoCompactFraction {
		return messages, false, nil
	}

	slog.Info("context pipeline: triggering compaction",
		slog.Float64("fraction", fraction),
		slog.Int("used", usedTokens),
		slog.Int("max", p.cfg.MaxTokens))

	compacted, _, err := p.RunFull(ctx, messages)
	if err != nil {
		return messages, false, err
	}
	return compacted, true, nil
}

// RunFull runs the LLM-based compaction and returns the compacted messages
// together with the generated summary text.
func (p *ContextPipeline) RunFull(ctx context.Context, messages []*Message) ([]*Message, string, error) {
	// Separate the tail (always kept) from the bulk to be summarised.
	keepN := p.cfg.KeepLast
	if keepN < 0 || keepN >= len(messages) {
		keepN = 0
	}
	bulk := messages
	tail := []*Message{}
	if keepN > 0 {
		bulk = messages[:len(messages)-keepN]
		tail = messages[len(messages)-keepN:]
	}

	compactedBulk, summary, err := CompactMessages(ctx, p.provider, bulk, p.model)
	if err != nil {
		return messages, "", err
	}

	result := append(compactedBulk, tail...)
	return result, summary, nil
}

// SnipOldMessages removes middle messages while preserving the first
// firstN and last lastN messages.  It is a cheap local pass run before
// falling back to LLM compaction.
func SnipOldMessages(messages []*Message, firstN, lastN int) []*Message {
	total := len(messages)
	keep := firstN + lastN
	if total <= keep {
		return messages
	}
	result := make([]*Message, 0, keep)
	result = append(result, messages[:firstN]...)
	result = append(result, messages[total-lastN:]...)
	return result
}
