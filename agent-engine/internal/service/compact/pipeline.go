package compact

import (
	"context"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

// PipelineConfig controls which passes run and their parameters.
type PipelineConfig struct {
	// MaxTokens is the target context window ceiling.  Compaction stops once
	// estimated token usage drops below CompactionFraction * MaxTokens.
	MaxTokens int
	// CompactionFraction is the target fraction to compact to (default 0.60).
	CompactionFraction float64
	// MicroMaxBlockChars is passed to MicroCompact (default 8000).
	MicroMaxBlockChars int
	// CollapseMaxChars is passed to CollapseToolResults (default 4000).
	CollapseMaxChars int
	// SnipOpts controls the Snip pass.
	SnipOpts SnipOptions
	// Model is the LLM model used for AutoCompact.
	Model string
	// DisableAutoCompact skips the LLM-driven pass even if still over budget.
	DisableAutoCompact bool
}

// PipelineResult summarises what the pipeline did.
type PipelineResult struct {
	TokensBefore int
	TokensAfter  int
	Summary      string   // non-empty only if AutoCompact ran
	PassesRun    []string // names of passes that executed
}

// RunPipeline executes the compact pipeline in order:
//
//  1. MicroCompact  – collapse whitespace, truncate huge text blocks
//  2. CollapseToolResults – snip oversized tool outputs
//  3. Snip          – remove middle messages beyond keep window
//  4. AutoCompact   – LLM summarisation (only if still over budget)
func RunPipeline(
	ctx context.Context,
	prov provider.Provider,
	messages []*engine.Message,
	cfg PipelineConfig,
) ([]*engine.Message, *PipelineResult, error) {

	if cfg.CompactionFraction <= 0 {
		cfg.CompactionFraction = 0.60
	}
	if cfg.MicroMaxBlockChars <= 0 {
		cfg.MicroMaxBlockChars = 8_000
	}
	if cfg.CollapseMaxChars <= 0 {
		cfg.CollapseMaxChars = 4_000
	}

	result := &PipelineResult{
		TokensBefore: estimateTokens(flattenText(messages)),
	}

	// ── Pass 1: MicroCompact ──────────────────────────────────────────────
	messages = MicroCompact(messages, cfg.MicroMaxBlockChars)
	result.PassesRun = append(result.PassesRun, "micro")

	// ── Pass 2: CollapseToolResults ───────────────────────────────────────
	messages = CollapseToolResults(messages, cfg.CollapseMaxChars)
	result.PassesRun = append(result.PassesRun, "collapse")

	// ── Pass 3: Snip ──────────────────────────────────────────────────────
	messages = Snip(messages, cfg.SnipOpts)
	result.PassesRun = append(result.PassesRun, "snip")

	// ── Pass 4: AutoCompact (only if still over budget) ───────────────────
	if !cfg.DisableAutoCompact && cfg.MaxTokens > 0 && prov != nil {
		est := estimateTokens(flattenText(messages))
		target := int(float64(cfg.MaxTokens) * cfg.CompactionFraction)
		if est > target {
			autoResult, err := RunAutoCompact(ctx, prov, messages, cfg.Model)
			if err != nil {
				return messages, result, fmt.Errorf("pipeline autocompact: %w", err)
			}
			result.Summary = autoResult.Summary
			result.PassesRun = append(result.PassesRun, "auto")
			messages = SummaryToMessages(autoResult.Summary)
		}
	}

	result.TokensAfter = estimateTokens(flattenText(messages))
	return messages, result, nil
}

// SummaryToMessages converts an auto-compact summary string into the standard
// two-message synthetic history (user context + assistant acknowledgement).
func SummaryToMessages(summary string) []*engine.Message {
	return []*engine.Message{
		{
			Role: engine.RoleUser,
			Content: []*engine.ContentBlock{{
				Type: engine.ContentTypeText,
				Text: "[Previous conversation summary]\n\n" + summary,
			}},
		},
		{
			Role: engine.RoleAssistant,
			Content: []*engine.ContentBlock{{
				Type: engine.ContentTypeText,
				Text: "I have reviewed the conversation summary and am ready to continue.",
			}},
		},
	}
}

// flattenText extracts all text from messages into a single string for token estimation.
func flattenText(messages []*engine.Message) string {
	var total int
	for _, m := range messages {
		for _, b := range m.Content {
			total += len(b.Text) + len(b.Thinking)
		}
	}
	buf := make([]byte, 0, total)
	for _, m := range messages {
		for _, b := range m.Content {
			buf = append(buf, b.Text...)
			buf = append(buf, b.Thinking...)
		}
	}
	return string(buf)
}
