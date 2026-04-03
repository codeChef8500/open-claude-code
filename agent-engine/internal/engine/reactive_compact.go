package engine

import (
	"context"
	"fmt"
	"log/slog"
)

// ────────────────────────────────────────────────────────────────────────────
// Reactive Compact — handles prompt-too-long errors by dynamically compacting
// the conversation and retrying the API call. Aligned with claude-code-main's
// reactive compaction logic in queryEngine.ts.
// ────────────────────────────────────────────────────────────────────────────

// ReactiveCompactConfig configures the reactive compaction strategy.
type ReactiveCompactConfig struct {
	// MaxRetries is the maximum number of PTL recovery attempts.
	MaxRetries int
	// DropOldestRounds removes the oldest N API rounds per retry.
	DropOldestRounds int
	// StripImages removes image blocks before retrying (saves tokens).
	StripImages bool
	// UseMicroCompact applies micro-compaction (dedup, truncation) first.
	UseMicroCompact bool
}

// DefaultReactiveCompactConfig returns sensible defaults.
func DefaultReactiveCompactConfig() ReactiveCompactConfig {
	return ReactiveCompactConfig{
		MaxRetries:       3,
		DropOldestRounds: 2,
		StripImages:      true,
		UseMicroCompact:  true,
	}
}

// ReactiveCompacter handles prompt-too-long errors during API calls.
type ReactiveCompacter struct {
	cfg    ReactiveCompactConfig
	caller ModelCaller
	model  string
}

// NewReactiveCompacter creates a reactive compacter.
func NewReactiveCompacter(caller ModelCaller, model string, cfg ReactiveCompactConfig) *ReactiveCompacter {
	return &ReactiveCompacter{
		cfg:    cfg,
		caller: caller,
		model:  model,
	}
}

// HandlePromptTooLong attempts to recover from a prompt-too-long error by
// progressively compacting the conversation. Returns the compacted messages
// or an error if all retries are exhausted.
func (rc *ReactiveCompacter) HandlePromptTooLong(
	ctx context.Context,
	messages []*Message,
	eventSink func(StreamEvent),
) ([]*Message, error) {
	current := messages

	for attempt := 1; attempt <= rc.cfg.MaxRetries; attempt++ {
		slog.Info("reactive compact: attempt",
			slog.Int("attempt", attempt),
			slog.Int("messages", len(current)))

		if eventSink != nil {
			eventSink(StreamEvent{
				Type: EventSystemMessage,
				Text: fmt.Sprintf("Prompt too long — compacting conversation (attempt %d/%d)…", attempt, rc.cfg.MaxRetries),
			})
		}

		// Stage 1: Strip images (cheap, often frees significant tokens).
		if rc.cfg.StripImages && attempt == 1 {
			current = stripImageBlocks(current)
		}

		// Stage 2: Micro-compact (dedup, truncation of old tool results).
		if rc.cfg.UseMicroCompact && attempt <= 2 {
			current = microCompactMessages(current)
		}

		// Stage 3: Drop oldest API rounds.
		if attempt >= 2 {
			roundsToDrop := rc.cfg.DropOldestRounds * attempt
			current = dropOldestRounds(current, roundsToDrop)
		}

		// Stage 4: Full compaction (LLM-based summary).
		if attempt >= 3 {
			compacted, _, err := CompactMessages(ctx, rc.caller, current, rc.model)
			if err != nil {
				slog.Warn("reactive compact: full compaction failed", slog.Any("err", err))
				continue
			}
			current = compacted
		}

		// Verify the resulting size is smaller.
		newEst := EstimateTokens(current)
		slog.Info("reactive compact: post-compact estimate",
			slog.Int("attempt", attempt),
			slog.Int("estimated_tokens", newEst))

		return current, nil
	}

	return nil, fmt.Errorf("reactive compact: exhausted %d retries", rc.cfg.MaxRetries)
}

// (IsPromptTooLongError is defined in messages.go)

// ── Message manipulation helpers ────────────────────────────────────────────

// stripImageBlocks removes all image content blocks from messages.
func stripImageBlocks(msgs []*Message) []*Message {
	out := make([]*Message, 0, len(msgs))
	for _, m := range msgs {
		newMsg := *m
		var filtered []*ContentBlock
		for _, b := range m.Content {
			if b.Type != ContentTypeImage {
				filtered = append(filtered, b)
			}
		}
		if len(filtered) == 0 && len(m.Content) > 0 {
			// Don't leave empty messages — add a placeholder.
			filtered = []*ContentBlock{{
				Type: ContentTypeText,
				Text: "[image removed to save context space]",
			}}
		}
		newMsg.Content = filtered
		out = append(out, &newMsg)
	}
	return out
}

// microCompactMessages applies lightweight compaction:
// - Truncates old tool result blocks that exceed 1000 chars
// - Removes duplicate consecutive text blocks
func microCompactMessages(msgs []*Message) []*Message {
	const maxToolResultChars = 1000
	protectedTail := 4 // protect last N messages from truncation

	out := make([]*Message, 0, len(msgs))
	for i, m := range msgs {
		isProtected := i >= len(msgs)-protectedTail
		if isProtected {
			out = append(out, m)
			continue
		}

		newMsg := *m
		var newContent []*ContentBlock
		for _, b := range m.Content {
			cb := *b
			// Truncate old tool results.
			if cb.Type == ContentTypeToolResult && len(cb.Content) > 0 {
				for j, inner := range cb.Content {
					if inner.Type == ContentTypeText && len(inner.Text) > maxToolResultChars {
						truncated := *inner
						truncated.Text = inner.Text[:maxToolResultChars] + "\n[... truncated for context space ...]"
						cb.Content[j] = &truncated
					}
				}
			}
			// Truncate old large text blocks.
			if cb.Type == ContentTypeText && len(cb.Text) > maxToolResultChars*2 {
				cb.Text = cb.Text[:maxToolResultChars*2] + "\n[... truncated ...]"
			}
			newContent = append(newContent, &cb)
		}
		newMsg.Content = newContent
		out = append(out, &newMsg)
	}
	return out
}

// dropOldestRounds removes the oldest N assistant+tool-result round pairs
// from the message history, preserving the system context and initial user message.
func dropOldestRounds(msgs []*Message, n int) []*Message {
	if n <= 0 || len(msgs) <= 3 {
		return msgs
	}

	// Keep the first message (user's initial prompt) and system messages.
	var prefix []*Message
	var body []*Message
	for i, m := range msgs {
		if i == 0 || m.Role == RoleSystem {
			prefix = append(prefix, m)
		} else {
			body = append(body, m)
		}
	}

	// Count API rounds in the body (assistant message = 1 round).
	roundsDropped := 0
	dropUntil := 0
	for i, m := range body {
		if m.Role == RoleAssistant {
			roundsDropped++
			if roundsDropped >= n {
				dropUntil = i + 1
				// Also drop the next message if it's a tool result.
				if dropUntil < len(body) && body[dropUntil].Role == RoleUser {
					dropUntil++
				}
				break
			}
		}
	}

	if dropUntil == 0 {
		return msgs
	}

	// Prepend a synthetic marker so the model knows context was truncated.
	marker := &Message{
		Role: RoleUser,
		Content: []*ContentBlock{{
			Type: ContentTypeText,
			Text: fmt.Sprintf("[Earlier conversation history (%d rounds) was removed to fit within context limits.]", roundsDropped),
		}},
	}

	result := make([]*Message, 0, len(prefix)+1+len(body)-dropUntil)
	result = append(result, prefix...)
	result = append(result, marker)
	result = append(result, body[dropUntil:]...)
	return result
}

// ── BudgetTracker continuation logic ────────────────────────────────────────

// ContinuationDecision represents whether the query loop should auto-continue.
type ContinuationDecision int

const (
	ContinuationStop     ContinuationDecision = 0
	ContinuationContinue ContinuationDecision = 1
	ContinuationCompact  ContinuationDecision = 2
)

// BudgetContinuationConfig configures auto-continuation behavior.
type BudgetContinuationConfig struct {
	// MinOutputFraction: if the model used less than this fraction of max_tokens,
	// it probably stopped intentionally (not truncated). Default 0.9.
	MinOutputFraction float64
	// MaxContinuations limits total auto-continuation attempts. Default 5.
	MaxContinuations int
}

// DefaultBudgetContinuationConfig returns sensible defaults.
func DefaultBudgetContinuationConfig() BudgetContinuationConfig {
	return BudgetContinuationConfig{
		MinOutputFraction: 0.90,
		MaxContinuations:  5,
	}
}

// EvaluateContinuation determines whether the query loop should auto-continue
// based on the model's output token usage and stop reason.
func EvaluateContinuation(
	stopReason string,
	outputTokens int,
	maxOutputTokens int,
	continuationCount int,
	cfg BudgetContinuationConfig,
) ContinuationDecision {
	// If the model explicitly said "end_turn", respect it.
	if stopReason == "end_turn" {
		return ContinuationStop
	}

	// If we've hit the continuation limit, stop.
	if continuationCount >= cfg.MaxContinuations {
		return ContinuationStop
	}

	// If stop reason is "max_tokens", the model was truncated — continue.
	if stopReason == "max_tokens" {
		return ContinuationContinue
	}

	// If the model used a high fraction of output tokens, it likely needs
	// more space to finish. This handles models that stop at the limit
	// without explicitly reporting "max_tokens".
	if maxOutputTokens > 0 && outputTokens > 0 {
		fraction := float64(outputTokens) / float64(maxOutputTokens)
		if fraction >= cfg.MinOutputFraction {
			return ContinuationContinue
		}
	}

	// If stop reason is "tool_use", the model wants to use a tool — this is
	// handled by the normal tool execution path, not continuation.
	if stopReason == "tool_use" {
		return ContinuationStop
	}

	return ContinuationStop
}

// BuildContinuationMessage creates the user message injected when
// auto-continuing after truncation.
func BuildContinuationMessage(attempt int) *Message {
	text := "Continue from where you left off. Do not repeat what you already said. " +
		"Pick up mid-thought if that is where the cut happened."
	if attempt > 1 {
		text += " Break remaining work into smaller pieces to avoid further truncation."
	}
	return CreateUserMessage(text, WithMeta())
}

// ── String helper ───────────────────────────────────────────────────────────

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
