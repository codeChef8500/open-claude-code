package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// ────────────────────────────────────────────────────────────────────────────
// Withheld error detection & recovery chain.
// Aligned with claude-code-main query.ts withheld error handling:
//   - isWithheldPromptTooLong
//   - isWithheldMaxOutputTokens
//   - isWithheldMediaSizeError
//   - tryReactiveCompact
//   - max_output_tokens escalation (ESCALATED_MAX_TOKENS)
// ────────────────────────────────────────────────────────────────────────────

// WithheldErrorType classifies a withheld API error.
type WithheldErrorType int

const (
	WithheldNone            WithheldErrorType = 0
	WithheldPromptTooLong   WithheldErrorType = 1
	WithheldMaxOutputTokens WithheldErrorType = 2
	WithheldMediaSizeError  WithheldErrorType = 3
)

// DetectWithheldError inspects an assistant message (including error metadata)
// to determine if it represents a withheld error that can be recovered from.
// Aligned with claude-code-main isWithheldPromptTooLong / isWithheldMaxOutputTokens /
// isWithheldMediaSizeError checks.
func DetectWithheldError(msg *Message) WithheldErrorType {
	if msg == nil {
		return WithheldNone
	}

	// Check for prompt-too-long (HTTP 413 or explicit API error).
	if msg.APIError != "" {
		lower := strings.ToLower(msg.APIError)
		if strings.Contains(lower, "prompt is too long") ||
			strings.Contains(lower, "prompt_too_long") ||
			strings.Contains(lower, "413") {
			return WithheldPromptTooLong
		}
		// Check for media/image size errors.
		if strings.Contains(lower, "image") && strings.Contains(lower, "size") ||
			strings.Contains(lower, "media") && strings.Contains(lower, "too large") {
			return WithheldMediaSizeError
		}
	}

	// Check for max_output_tokens via stop_reason.
	if msg.StopReason == "max_tokens" {
		return WithheldMaxOutputTokens
	}

	return WithheldNone
}

// ── Recovery chain ───────────────────────────────────────────────────────

// RecoveryConfig holds configuration for the withheld error recovery chain.
type RecoveryConfig struct {
	// Model is the current model name.
	Model string
	// ContextWindowSize is the max context tokens.
	ContextWindowSize int
	// ReactiveCompactEnabled enables reactive compact for PTL recovery.
	ReactiveCompactEnabled bool
	// ContextCollapseEnabled enables context collapse drain for PTL recovery.
	ContextCollapseEnabled bool
}

// RecoveryAction describes what the query loop should do after detecting a
// withheld error.
type RecoveryAction struct {
	// Transition is the continue reason for the next iteration.
	Transition *ContinueTransition
	// Messages is the (possibly modified) message list.
	Messages []*Message
	// MaxOutputTokensOverride, if non-nil, overrides max output tokens.
	MaxOutputTokensOverride *int
	// SystemMessage is an optional system message to emit.
	SystemMessage string
	// IsFatal is true if recovery failed and the loop should terminate.
	IsFatal bool
	// FatalError is set when IsFatal is true.
	FatalError error
}

// HandleWithheldPromptTooLong attempts PTL recovery through a 3-stage chain:
//  1. Context collapse drain (if enabled) — yields ContinueCollapseDrainRetry
//  2. Reactive compact (if enabled) — yields ContinueReactiveCompactRetry
//  3. Fatal — cannot recover
//
// Aligned with claude-code-main query.ts PTL recovery block.
func HandleWithheldPromptTooLong(
	ctx context.Context,
	ls *loopState,
	caller ModelCaller,
	cfg RecoveryConfig,
	out chan<- *StreamEvent,
) *RecoveryAction {
	slog.Warn("withheld: prompt-too-long detected",
		slog.Int("turn", ls.turnCount))

	// Stage 1: Context collapse drain (not yet implemented, placeholder).
	if cfg.ContextCollapseEnabled && !ls.hasAttemptedReactiveCompact {
		slog.Info("withheld: attempting context collapse drain")
		// TODO: Wire to contextcollapse.RecoverFromOverflow when available.
		// For now, fall through to reactive compact.
	}

	// Stage 2: Reactive compact.
	if cfg.ReactiveCompactEnabled && !ls.hasAttemptedReactiveCompact {
		slog.Info("withheld: attempting reactive compact for PTL recovery")
		ls.hasAttemptedReactiveCompact = true

		rc := NewReactiveCompacter(caller, cfg.Model, DefaultReactiveCompactConfig())
		compacted, err := rc.HandlePromptTooLong(ctx, ls.messages, func(evt StreamEvent) {
			out <- &evt
		})
		if err != nil {
			slog.Warn("withheld: reactive compact failed", slog.Any("err", err))
		} else {
			return &RecoveryAction{
				Transition: &ContinueTransition{
					Reason: ContinueReactiveCompactRetry,
				},
				Messages:      compacted,
				SystemMessage: "Context was too large. Compacted conversation and retrying.",
			}
		}
	}

	// Stage 3: Fatal — cannot recover.
	return &RecoveryAction{
		IsFatal:    true,
		FatalError: fmt.Errorf("prompt too long: unable to recover after compaction attempts"),
		Messages:   ls.messages,
	}
}

// HandleWithheldMaxOutputTokens attempts max_output_tokens recovery through:
//  1. OTK escalation — increase max tokens to escalatedMaxTokens
//  2. Multi-turn recovery — inject recovery message and retry (up to limit)
//  3. Fatal — recovery limit exceeded
//
// Aligned with claude-code-main query.ts max_output_tokens recovery.
func HandleWithheldMaxOutputTokens(
	ls *loopState,
	currentMaxTokens int,
) *RecoveryAction {
	slog.Warn("withheld: max_output_tokens detected",
		slog.Int("turn", ls.turnCount),
		slog.Int("recovery_count", ls.maxOutputTokensRecoveryCount))

	// Stage 1: OTK escalation — try escalated limit first.
	if ls.maxOutputTokensOverride == nil && currentMaxTokens < escalatedMaxTokens {
		slog.Info("withheld: escalating max_output_tokens",
			slog.Int("from", currentMaxTokens),
			slog.Int("to", escalatedMaxTokens))
		override := escalatedMaxTokens
		return &RecoveryAction{
			Transition: &ContinueTransition{
				Reason: ContinueMaxOutputTokensEscalate,
			},
			Messages:                ls.messages,
			MaxOutputTokensOverride: &override,
			SystemMessage:           "Output was truncated. Escalating output token limit and retrying.",
		}
	}

	// Stage 2: Multi-turn recovery — inject a recovery message.
	if ls.maxOutputTokensRecoveryCount < maxOutputTokensRecoveryLimit {
		ls.maxOutputTokensRecoveryCount++
		slog.Info("withheld: multi-turn max_tokens recovery",
			slog.Int("attempt", ls.maxOutputTokensRecoveryCount))

		recoveryMsg := &Message{
			UUID: uuid.New().String(),
			Role: RoleUser,
			Content: []*ContentBlock{{
				Type: ContentTypeText,
				Text: "Output token limit hit. Resume directly — no apology, no recap of what you were doing. " +
					"Pick up mid-thought if that is where the cut happened. Break remaining work into smaller pieces.",
			}},
		}

		msgs := append(ls.messages, recoveryMsg)
		return &RecoveryAction{
			Transition: &ContinueTransition{
				Reason:  ContinueMaxOutputTokensRecovery,
				Attempt: ls.maxOutputTokensRecoveryCount,
			},
			Messages: msgs,
		}
	}

	// Stage 3: Fatal — recovery limit exceeded.
	return &RecoveryAction{
		IsFatal:    true,
		FatalError: fmt.Errorf("max_output_tokens recovery limit exceeded (%d attempts)", maxOutputTokensRecoveryLimit),
		Messages:   ls.messages,
	}
}

// HandleWithheldMediaSizeError attempts media size error recovery by running
// reactive compact (strip images).  Shares the same recovery path as PTL.
func HandleWithheldMediaSizeError(
	ctx context.Context,
	ls *loopState,
	caller ModelCaller,
	cfg RecoveryConfig,
	out chan<- *StreamEvent,
) *RecoveryAction {
	slog.Warn("withheld: media size error detected",
		slog.Int("turn", ls.turnCount))

	// Use reactive compact to strip images and retry.
	if cfg.ReactiveCompactEnabled && !ls.hasAttemptedReactiveCompact {
		ls.hasAttemptedReactiveCompact = true
		rc := NewReactiveCompacter(caller, cfg.Model, ReactiveCompactConfig{
			MaxRetries:  2,
			StripImages: true,
		})
		compacted, err := rc.HandlePromptTooLong(ctx, ls.messages, func(evt StreamEvent) {
			out <- &evt
		})
		if err != nil {
			slog.Warn("withheld: reactive compact for media error failed", slog.Any("err", err))
		} else {
			return &RecoveryAction{
				Transition: &ContinueTransition{
					Reason: ContinueReactiveCompactRetry,
				},
				Messages:      compacted,
				SystemMessage: "Image was too large. Stripped images and retrying.",
			}
		}
	}

	return &RecoveryAction{
		IsFatal:    true,
		FatalError: fmt.Errorf("media size error: unable to recover"),
		Messages:   ls.messages,
	}
}

// ── Token warning / blocking limit ───────────────────────────────────────

// CheckTokenBlockingLimit determines if the context is too full to make an
// API call.  Returns true if the loop should skip the API call and trigger
// compaction instead.
// Aligned with claude-code-main calculateTokenWarningState + isAtBlockingLimit.
func CheckTokenBlockingLimit(
	budgetState TokenBudgetState,
	querySource QuerySource,
	reactiveCompactEnabled bool,
) bool {
	// Never block for compact or session_memory queries — they are the
	// compactors themselves.
	if querySource.IsCompactOrSessionMemory() {
		return false
	}

	// Use the existing warning level calculation.
	level := CalculateTokenWarningState(budgetState.InputTokens, budgetState.ContextWindowSize)
	return level == TokenWarningBlocking
}
