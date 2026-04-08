package engine

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/util"
)

// ────────────────────────────────────────────────────────────────────────────
// Compression Pipeline — the 5-step context compression pipeline executed at
// the start of each queryLoop iteration, before the API call.
// Aligned with claude-code-main query.ts iteration entry.
//
// Order:
//   1. getMessagesAfterCompactBoundary — trim to post-boundary messages
//   2. applyToolResultBudget          — truncate oversized tool results
//   3. snipCompactIfNeeded            — remove stale middle history (feature: HISTORY_SNIP)
//   4. microcompact                   — dedup + truncate old tool results
//   5. autocompact                    — LLM-based summarisation when context is full
//
// contextCollapse is omitted for now (requires a separate sub-package).
// ────────────────────────────────────────────────────────────────────────────

// CompressionPipelineConfig holds configuration for the compression pipeline.
type CompressionPipelineConfig struct {
	// DisableCompaction skips the entire pipeline.
	DisableCompaction bool
	// QuerySource is the origin of the current query (compact/session_memory skip some steps).
	QuerySource QuerySource
	// Flags is the feature flag store for gate checks.
	Flags *util.FeatureFlagStore
	// Model is the model name for LLM-based compaction.
	Model string
	// ContextWindowSize is the maximum context window tokens.
	ContextWindowSize int
}

// CompressionPipelineResult holds the output of the compression pipeline.
type CompressionPipelineResult struct {
	// Messages is the (possibly compressed) conversation history.
	Messages []*Message
	// SnipTokensFreed is the estimated tokens freed by snip pass.
	SnipTokensFreed int
	// SnipBoundaryMessage is the boundary message emitted by snip (may be nil).
	SnipBoundaryMessage *Message
	// MicrocompactInfo holds micro-compaction details (may be nil).
	MicrocompactInfo *MicrocompactInfo
	// CompactionResult holds auto-compaction details (may be nil).
	CompactionResult *CompactionResult
	// CompactBoundaryMessages are system messages emitted by compaction.
	CompactBoundaryMessages []*Message
}

// RunCompressionPipeline executes the full 5-step compression pipeline.
// It returns the processed messages and metadata about what was done.
func RunCompressionPipeline(
	ctx context.Context,
	messages []*Message,
	caller ModelCaller,
	deps *QueryDeps,
	cfg CompressionPipelineConfig,
) (*CompressionPipelineResult, error) {
	result := &CompressionPipelineResult{}

	if cfg.DisableCompaction {
		result.Messages = messages
		return result, nil
	}

	// ── Step 1: getMessagesAfterCompactBoundary ──────────────────────────
	msgs := getMessagesAfterCompactBoundary(messages)

	// ── Step 2: applyToolResultBudget ────────────────────────────────────
	// Delegate to the existing ToolResultBudget system if available.
	// This truncates oversized tool results inline.
	msgs = applyToolResultBudgetToMessages(msgs)

	// ── Step 3: snipCompactIfNeeded (feature: HISTORY_SNIP) ─────────────
	if cfg.Flags != nil && cfg.Flags.IsEnabled(util.FlagHistorySnip) {
		snipped, tokensFreed, boundaryMsg := snipCompactIfNeeded(msgs)
		if tokensFreed > 0 {
			msgs = snipped
			result.SnipTokensFreed = tokensFreed
			result.SnipBoundaryMessage = boundaryMsg
			slog.Debug("compression_pipeline: snip freed tokens",
				slog.Int("tokens_freed", tokensFreed))
		}
	}

	// ── Step 4: microcompact ─────────────────────────────────────────────
	if cfg.Flags != nil && cfg.Flags.IsEnabled(util.FlagMicroCompact) {
		mcCfg := MicrocompactConfig{
			ProtectLastN:       4,
			MaxToolResultChars: 50000,
		}
		var mcMsgs []*Message
		var mcInfo *MicrocompactInfo
		var err error

		if deps != nil && deps.MicrocompactOverride != nil {
			mcMsgs, mcInfo, err = deps.MicrocompactOverride(msgs, mcCfg)
		} else {
			mcMsgs, mcInfo, err = defaultMicrocompact(msgs, mcCfg)
		}
		if err != nil {
			slog.Warn("compression_pipeline: microcompact failed", slog.Any("err", err))
		} else {
			msgs = mcMsgs
			result.MicrocompactInfo = mcInfo
		}
	}

	// ── Step 5: autocompact ──────────────────────────────────────────────
	// Skip auto-compact for compact/session_memory sources (they are the
	// compactors themselves and would deadlock).
	if cfg.QuerySource.IsCompactOrSessionMemory() {
		result.Messages = msgs
		return result, nil
	}

	if cfg.Flags != nil && cfg.Flags.IsEnabled(util.FlagAutoCompact) {
		shouldCompact := shouldAutoCompact(msgs, cfg.ContextWindowSize)
		if shouldCompact {
			slog.Info("compression_pipeline: auto-compact triggered")
			acCfg := AutocompactConfig{
				Model:           cfg.Model,
				MaxOutputTokens: 16384,
			}
			var compResult *CompactionResult
			var err error

			if deps != nil && deps.AutocompactOverride != nil {
				compResult, err = deps.AutocompactOverride(ctx, msgs, caller, cfg.Model, acCfg)
			} else {
				compResult, err = defaultAutocompact(ctx, msgs, caller, cfg.Model)
			}
			if err != nil {
				slog.Warn("compression_pipeline: auto-compact failed", slog.Any("err", err))
			} else if compResult != nil {
				result.CompactionResult = compResult
				msgs = buildPostCompactMessages(compResult)
				result.CompactBoundaryMessages = buildCompactBoundaryMessages(compResult)
			}
		}
	}

	result.Messages = msgs
	return result, nil
}

// ── Step 1 helper ────────────────────────────────────────────────────────

// getMessagesAfterCompactBoundary returns messages after the last compact
// boundary, or all messages if no boundary exists.
// Aligned with claude-code-main getMessagesAfterCompactBoundary.
func getMessagesAfterCompactBoundary(messages []*Message) []*Message {
	lastBoundaryIdx := -1
	for i, m := range messages {
		if m.Type == MsgTypeCompactBoundary {
			lastBoundaryIdx = i
		}
	}
	if lastBoundaryIdx < 0 {
		return messages
	}
	// Include the boundary itself + everything after.
	return messages[lastBoundaryIdx:]
}

// ── Step 2 helper ────────────────────────────────────────────────────────

// applyToolResultBudgetToMessages truncates oversized tool result content
// blocks inline.  Aligned with claude-code-main applyToolResultBudget.
func applyToolResultBudgetToMessages(messages []*Message) []*Message {
	const maxToolResultChars = 100_000

	for _, msg := range messages {
		if msg.Role != RoleUser {
			continue
		}
		for _, block := range msg.Content {
			if block.Type != ContentTypeToolResult {
				continue
			}
			// Truncate nested text content within tool results.
			for _, nested := range block.Content {
				if nested.Type == ContentTypeText && len(nested.Text) > maxToolResultChars {
					nested.Text = nested.Text[:maxToolResultChars] + "\n... [truncated]"
				}
			}
		}
	}
	return messages
}

// ── Step 3 helper ────────────────────────────────────────────────────────

// snipCompactIfNeeded removes old middle messages while preserving the first
// and last segments.  Returns the snipped messages, tokens freed, and an
// optional boundary message.
func snipCompactIfNeeded(messages []*Message) ([]*Message, int, *Message) {
	const preserveFirst = 2
	const preserveLast = 6

	total := len(messages)
	if total <= preserveFirst+preserveLast {
		return messages, 0, nil
	}

	// Check if there are old tool results in the middle that can be snipped.
	middle := messages[preserveFirst : total-preserveLast]
	snipCount := 0
	for _, m := range middle {
		if m.Role == RoleUser || m.Role == RoleAssistant {
			snipCount++
		}
	}
	if snipCount == 0 {
		return messages, 0, nil
	}

	// Build snipped result: first + last.
	result := make([]*Message, 0, preserveFirst+preserveLast+1)
	result = append(result, messages[:preserveFirst]...)

	// Insert a snip boundary marker.
	boundary := &Message{
		UUID:  uuid.New().String(),
		Role:  RoleSystem,
		Type:  MsgTypeSystem,
		Level: SystemLevelInfo,
		Content: []*ContentBlock{{
			Type: ContentTypeText,
			Text: "[Older conversation history has been snipped to save context space]",
		}},
	}
	result = append(result, boundary)
	result = append(result, messages[total-preserveLast:]...)

	// Rough token estimate: ~4 chars per token.
	tokensFreed := 0
	for _, m := range middle {
		for _, b := range m.Content {
			tokensFreed += len(b.Text) / 4
		}
	}

	return result, tokensFreed, boundary
}

// ── Step 4 helper ────────────────────────────────────────────────────────

// defaultMicrocompact is the built-in micro-compaction implementation.
// It deduplicates and truncates old tool results to reclaim context.
func defaultMicrocompact(messages []*Message, cfg MicrocompactConfig) ([]*Message, *MicrocompactInfo, error) {
	protectLast := cfg.ProtectLastN
	if protectLast <= 0 {
		protectLast = 4
	}
	maxChars := cfg.MaxToolResultChars
	if maxChars <= 0 {
		maxChars = 50000
	}

	info := &MicrocompactInfo{}
	total := len(messages)

	for i, msg := range messages {
		// Only process messages outside the protected tail.
		if i >= total-protectLast {
			break
		}
		if msg.Role != RoleUser {
			continue
		}
		for _, block := range msg.Content {
			if block.Type != ContentTypeToolResult {
				continue
			}
			for _, nested := range block.Content {
				if nested.Type == ContentTypeText && len(nested.Text) > maxChars {
					freed := len(nested.Text) - maxChars
					nested.Text = nested.Text[:maxChars] + "\n... [microcompact truncated]"
					info.TokensFreed += freed / 4
					info.EntriesRemoved++
				}
			}
		}
	}

	return messages, info, nil
}

// ── Step 5 helpers ───────────────────────────────────────────────────────

// shouldAutoCompact returns true if token usage exceeds the auto-compact
// threshold (85% of context window).
func shouldAutoCompact(messages []*Message, contextWindowSize int) bool {
	if contextWindowSize <= 0 {
		return false
	}
	estimated := estimateMessageTokens(messages)
	threshold := float64(contextWindowSize) * 0.85
	return float64(estimated) >= threshold
}

// estimateMessageTokens provides a rough token estimate for a message slice.
func estimateMessageTokens(messages []*Message) int {
	total := 0
	for _, m := range messages {
		for _, b := range m.Content {
			total += len(b.Text)/4 + 10 // ~4 chars/token + overhead
			if b.Type == ContentTypeToolResult {
				for _, nested := range b.Content {
					total += len(nested.Text)/4 + 5
				}
			}
		}
	}
	return total
}

// defaultAutocompact runs the built-in LLM-based auto-compaction using the
// engine's existing CompactMessages function.
func defaultAutocompact(ctx context.Context, messages []*Message, caller ModelCaller, model string) (*CompactionResult, error) {
	// Preserve recent messages.
	const keepLast = 10
	if len(messages) <= keepLast {
		return nil, nil
	}

	bulk := messages[:len(messages)-keepLast]
	tail := messages[len(messages)-keepLast:]

	compacted, summary, err := CompactMessages(ctx, caller, bulk, model)
	if err != nil {
		return nil, err
	}

	result := &CompactionResult{
		SummaryMessages:      compacted,
		MessagesToKeep:       tail,
		PreCompactTokenCount: estimateMessageTokens(messages),
		UserDisplayMessage:   summary,
	}
	result.PostCompactTokenCount = estimateMessageTokens(append(compacted, tail...))

	return result, nil
}

// buildPostCompactMessages combines summary messages with kept tail messages.
func buildPostCompactMessages(result *CompactionResult) []*Message {
	out := make([]*Message, 0, len(result.SummaryMessages)+len(result.MessagesToKeep))
	out = append(out, result.SummaryMessages...)
	out = append(out, result.MessagesToKeep...)
	return out
}

// buildCompactBoundaryMessages creates system messages marking compaction.
func buildCompactBoundaryMessages(result *CompactionResult) []*Message {
	if result == nil {
		return nil
	}
	boundary := &Message{
		UUID:             uuid.New().String(),
		Role:             RoleSystem,
		Type:             MsgTypeCompactBoundary,
		IsCompactSummary: true,
		Content: []*ContentBlock{{
			Type: ContentTypeText,
			Text: result.UserDisplayMessage,
		}},
	}
	return []*Message{boundary}
}
