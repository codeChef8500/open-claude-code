package command

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// /compact — full implementation
// Aligned with claude-code-main commands/compact/compact.ts.
//
// The compact command performs conversation compaction to free context space:
//  1. Get messages after compact boundary
//  2. Try session memory compaction (if no custom instructions)
//  3. If reactive-only mode, use reactive path
//  4. Otherwise: microcompact → full compaction
//  5. Reset lastSummarizedMessageId
//  6. Clear user context cache + post-compact cleanup
//  7. Build display text (upgrade hints, shortcut hints)
//  8. Return {type: compact, compactionResult, displayText}
// ──────────────────────────────────────────────────────────────────────────────

// CompactConversation performs the full conversation compaction sequence.
// This is the Go equivalent of claude-code-main's compact command call().
func CompactConversation(ctx context.Context, args []string, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "__compact__", nil
	}

	svc := ectx.Services
	if svc == nil || svc.Compact == nil {
		return "__compact__", nil
	}

	// Parse custom instruction from args.
	customInstruction := strings.Join(args, " ")

	// Build compact options from current context.
	opts := CompactOptions{
		Messages:          ectx.Messages,
		Model:             ectx.Model,
		CustomInstruction: customInstruction,
	}

	var result *CompactResult
	var err error

	// Step 1: Try session memory compaction (only when no custom instruction).
	if customInstruction == "" {
		var ok bool
		result, ok, err = svc.Compact.TrySessionMemoryCompaction(ctx, opts)
		if err != nil {
			return "", fmt.Errorf("session memory compaction failed: %w", err)
		}
		if ok && result != nil {
			return formatCompactResult(result, ectx), nil
		}
	}

	// Step 2: Microcompact pass — remove redundant tool output.
	if svc.Compact != nil && opts.Messages != nil {
		microcompacted, merr := svc.Compact.MicrocompactMessages(ctx, opts.Messages)
		if merr == nil && microcompacted != nil {
			opts.Messages = microcompacted
		}
	}

	// Step 3: Full compaction.
	result, err = svc.Compact.CompactConversation(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("compaction failed: %w", err)
	}

	// Step 4: Post-compaction cleanup.
	if svc.Cache != nil {
		svc.Cache.ClearUserContextCache()
		svc.Cache.ClearPostCompactCleanup()
	}

	// Step 5: Update messages if SetMessages is available.
	if result != nil && result.Messages != nil && ectx.SetMessages != nil {
		newMsgs := result.Messages
		ectx.SetMessages(func(_ interface{}) interface{} {
			return newMsgs
		})
	}

	return formatCompactResult(result, ectx), nil
}

// formatCompactResult builds the display text for a compaction result.
// Aligned with claude-code-main compact command's return value construction.
func formatCompactResult(result *CompactResult, ectx *ExecContext) string {
	if result == nil {
		return "__compact__"
	}

	var parts []string
	parts = append(parts, "__compact__")

	// Token savings summary.
	if result.TokensBefore > 0 && result.TokensAfter > 0 {
		saved := result.TokensBefore - result.TokensAfter
		pct := float64(saved) / float64(result.TokensBefore) * 100
		parts = append(parts, fmt.Sprintf(
			"Compacted: %d → %d tokens (saved %d, %.0f%%)",
			result.TokensBefore, result.TokensAfter, saved, pct,
		))
	}

	// Strategy used.
	if result.Strategy != "" {
		parts = append(parts, fmt.Sprintf("Strategy: %s", result.Strategy))
	}

	// Summary preview.
	if result.Summary != "" {
		summary := result.Summary
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		parts = append(parts, fmt.Sprintf("Summary: %s", summary))
	}

	// Shortcut hints.
	parts = append(parts, "Tip: Use Shift+Cmd+K to compact at any time.")

	return strings.Join(parts, "\n")
}

// ──────────────────────────────────────────────────────────────────────────────
// Reactive compaction — triggered when prompt is too long.
// Aligned with claude-code-main services/compact/reactiveCompact.ts.
// ──────────────────────────────────────────────────────────────────────────────

// ReactiveCompactOnPromptTooLong performs reactive compaction when the prompt
// exceeds the model's context window. This is called by the engine, not
// directly by the /compact command.
func ReactiveCompactOnPromptTooLong(ctx context.Context, ectx *ExecContext) (*CompactResult, error) {
	if ectx == nil || ectx.Services == nil || ectx.Services.Compact == nil {
		return nil, fmt.Errorf("compact service not available")
	}

	opts := CompactOptions{
		Messages: ectx.Messages,
		Model:    ectx.Model,
	}

	return ectx.Services.Compact.ReactiveCompact(ctx, opts)
}
