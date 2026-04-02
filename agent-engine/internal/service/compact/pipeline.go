package compact

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/hooks"
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
	// HookExecutor fires PreCompact/PostCompact hooks if configured.
	HookExecutor *hooks.Executor
	// CustomInstructions are user-provided custom compact instructions.
	CustomInstructions string
	// IsAutoCompact distinguishes auto-compact from manual /compact.
	IsAutoCompact bool
	// MaxPTLRetries is the maximum prompt-too-long retries (default 3).
	MaxPTLRetries int
	// MicroCompactConfig overrides MicroCompact defaults.
	MicroCompactCfg *MicroCompactConfig
}

// PipelineResult summarises what the pipeline did.
type PipelineResult struct {
	TokensBefore int
	TokensAfter  int
	Summary      string   // non-empty only if AutoCompact ran
	PassesRun    []string // names of passes that executed
	// CompactionResult is the full result when AutoCompact ran.
	CompactionResult *CompactionResult
}

// RunPipeline executes the compact pipeline in order:
//
//  1. StripImages   – remove images before compaction
//  2. MicroCompact  – collapse whitespace, truncate huge text blocks
//  3. CollapseToolResults – snip oversized tool outputs
//  4. Snip          – remove middle messages beyond keep window
//  5. AutoCompact   – LLM summarisation (only if still over budget)
//
// Hooks are fired at the beginning (PreCompact) and end (PostCompact)
// if a HookExecutor is configured.
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
	if cfg.MaxPTLRetries <= 0 {
		cfg.MaxPTLRetries = 3
	}

	result := &PipelineResult{
		TokensBefore: estimateTokens(flattenText(messages)),
	}

	// ── Fire PreCompact hook ────────────────────────────────────────────
	customInstructions := cfg.CustomInstructions
	if cfg.HookExecutor != nil && cfg.HookExecutor.HasHooksFor(hooks.EventPreCompact) {
		trigger := "manual"
		if cfg.IsAutoCompact {
			trigger = "auto"
		}
		hookResp := cfg.HookExecutor.RunSync(ctx, hooks.EventPreCompact, &hooks.HookInput{
			PreCompact: &hooks.PreCompactInput{
				Trigger:            trigger,
				CustomInstructions: customInstructions,
			},
		})
		if hookResp.NewCustomInstructions != "" {
			customInstructions = MergeHookInstructions(customInstructions, hookResp.NewCustomInstructions)
		}
	}

	// ── Pass 0: StripImages ─────────────────────────────────────────────
	messages = StripImagesFromMessages(messages)
	result.PassesRun = append(result.PassesRun, "strip_images")

	// ── Pass 1: MicroCompact ──────────────────────────────────────────────
	if cfg.MicroCompactCfg != nil {
		messages = MicroCompactWithConfig(messages, *cfg.MicroCompactCfg)
	} else {
		messages = MicroCompact(messages, cfg.MicroMaxBlockChars)
	}
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
			// PTL retry loop: if the compact call itself hits prompt-too-long,
			// truncate oldest groups and retry.
			var autoResult *AutoCompactResult
			var autoErr error
			msgsToSummarize := messages
			for attempt := 0; attempt <= cfg.MaxPTLRetries; attempt++ {
				autoResult, autoErr = RunAutoCompact(ctx, prov, msgsToSummarize, cfg.Model, customInstructions)
				if autoErr == nil {
					break
				}
				// Check if the error looks like prompt-too-long.
				if !isPromptTooLongError(autoErr) || attempt >= cfg.MaxPTLRetries {
					break
				}
				slog.Info("pipeline: PTL retry",
					slog.Int("attempt", attempt+1),
					slog.Int("messages", len(msgsToSummarize)))
				truncated := TruncateHeadForPTLRetry(msgsToSummarize, 0)
				if truncated == nil {
					break // nothing left to drop
				}
				msgsToSummarize = truncated
			}
			if autoErr != nil {
				return messages, result, fmt.Errorf("pipeline autocompact: %w", autoErr)
			}
			result.Summary = autoResult.Summary
			result.PassesRun = append(result.PassesRun, "auto")
			result.CompactionResult = &CompactionResult{
				SummaryMessages:       SummaryToMessages(autoResult.Summary),
				PreCompactTokenCount:  autoResult.TokensBefore,
				PostCompactTokenCount: autoResult.TokensAfter,
				CompactionUsage:       autoResult.Usage,
			}
			messages = result.CompactionResult.SummaryMessages
		}
	}

	result.TokensAfter = estimateTokens(flattenText(messages))
	return messages, result, nil
}

// isPromptTooLongError checks if an error looks like a prompt-too-long API error.
func isPromptTooLongError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "prompt_too_long") ||
		strings.Contains(msg, "maximum context length")
}

// CompactConversation is the high-level entry point for compacting a conversation.
// It runs the full pipeline with hooks, image stripping, and PTL retry.
func CompactConversation(
	ctx context.Context,
	prov provider.Provider,
	messages []*engine.Message,
	cfg PipelineConfig,
) (*CompactionResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("not enough messages to compact")
	}

	preTokens := estimateTokensFromMessages(messages)

	newMsgs, pipeResult, err := RunPipeline(ctx, prov, messages, cfg)
	if err != nil {
		return nil, err
	}

	// If the pipeline produced a CompactionResult (auto-compact ran), use it.
	if pipeResult.CompactionResult != nil {
		pipeResult.CompactionResult.PreCompactTokenCount = preTokens
		pipeResult.CompactionResult.TruePostCompactTokenCount = estimateTokensFromMessages(newMsgs)
		return pipeResult.CompactionResult, nil
	}

	// Otherwise build a result from the pipeline output.
	postTokens := estimateTokensFromMessages(newMsgs)
	return &CompactionResult{
		SummaryMessages:           newMsgs,
		PreCompactTokenCount:      preTokens,
		PostCompactTokenCount:     postTokens,
		TruePostCompactTokenCount: postTokens,
	}, nil
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
