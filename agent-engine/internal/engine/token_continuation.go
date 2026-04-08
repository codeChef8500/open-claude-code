package engine

import (
	"log/slog"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Token budget continuation — decides whether the agent should keep running
// after the model stops, based on token usage patterns and diminishing returns.
// Aligned with claude-code-main query/tokenBudget.ts checkTokenBudget.
// ────────────────────────────────────────────────────────────────────────────

// TokenBudgetDecision is the output of CheckTokenBudgetContinuation.
type TokenBudgetDecision struct {
	// ShouldContinue is true if the loop should auto-continue.
	ShouldContinue bool
	// Reason describes why continuation was approved or rejected.
	Reason string
	// ContinuationCount is the updated continuation counter.
	ContinuationCount int
}

// BudgetContinuationTracker tracks state for the diminishing-returns heuristic.
// Aligned with claude-code-main BudgetTracker.
type BudgetContinuationTracker struct {
	// ContinuationCount tracks how many times we auto-continued.
	ContinuationCount int
	// LastDeltaTokens is the output token delta from the last iteration.
	LastDeltaTokens int
	// LastGlobalTurnTokens is the total turn tokens from the last iteration.
	LastGlobalTurnTokens int
	// StartedAt is when the query started.
	StartedAt time.Time
	// InitialOutputTokens is the output tokens at the start of the chain.
	InitialOutputTokens int
}

// NewBudgetContinuationTracker creates a tracker.
func NewBudgetContinuationTracker() *BudgetContinuationTracker {
	return &BudgetContinuationTracker{
		StartedAt: time.Now(),
	}
}

// CheckTokenBudgetContinuation decides whether the query loop should
// auto-continue after the model produces a stop_reason of "end_turn"
// (no tool calls).
//
// The logic mirrors claude-code-main's checkTokenBudget:
//   - If output tokens are below a minimum threshold, don't continue (no work done).
//   - If the ratio of new-to-total output tokens is declining (diminishing returns),
//     don't continue.
//   - If max continuations reached, don't continue.
//   - Otherwise, continue.
//
// This is used for "token budget continuation" where the model is expected to
// keep working until the budget is exhausted.
func CheckTokenBudgetContinuation(
	tracker *BudgetContinuationTracker,
	currentOutputTokens int,
	maxOutputTokens int,
	stopReason string,
) *TokenBudgetDecision {
	const (
		maxContinuations       = 5
		minOutputTokenThreshold = 200
		diminishingReturnRatio  = 0.10 // if delta/total < 10%, diminishing returns
	)

	// Only continue on "end_turn" — not max_tokens or tool_use.
	if stopReason != "end_turn" {
		return &TokenBudgetDecision{
			ShouldContinue:    false,
			Reason:            "stop_reason is not end_turn: " + stopReason,
			ContinuationCount: tracker.ContinuationCount,
		}
	}

	// Check continuation limit.
	if tracker.ContinuationCount >= maxContinuations {
		return &TokenBudgetDecision{
			ShouldContinue:    false,
			Reason:            "max continuations reached",
			ContinuationCount: tracker.ContinuationCount,
		}
	}

	// Check if enough output was produced.
	delta := currentOutputTokens - tracker.LastGlobalTurnTokens
	if delta < minOutputTokenThreshold {
		return &TokenBudgetDecision{
			ShouldContinue:    false,
			Reason:            "output delta below threshold",
			ContinuationCount: tracker.ContinuationCount,
		}
	}

	// Check diminishing returns.
	totalOutput := currentOutputTokens - tracker.InitialOutputTokens
	if totalOutput > 0 {
		ratio := float64(delta) / float64(totalOutput)
		if ratio < diminishingReturnRatio {
			slog.Info("token_continuation: diminishing returns detected",
				slog.Float64("ratio", ratio),
				slog.Int("delta", delta),
				slog.Int("total", totalOutput))
			return &TokenBudgetDecision{
				ShouldContinue:    false,
				Reason:            "diminishing returns",
				ContinuationCount: tracker.ContinuationCount,
			}
		}
	}

	// Check if we still have budget headroom.
	if maxOutputTokens > 0 && currentOutputTokens >= maxOutputTokens*9/10 {
		return &TokenBudgetDecision{
			ShouldContinue:    false,
			Reason:            "near output token limit",
			ContinuationCount: tracker.ContinuationCount,
		}
	}

	// Continue.
	tracker.ContinuationCount++
	tracker.LastDeltaTokens = delta
	tracker.LastGlobalTurnTokens = currentOutputTokens

	slog.Info("token_continuation: auto-continuing",
		slog.Int("continuation", tracker.ContinuationCount),
		slog.Int("delta_tokens", delta),
		slog.Int("total_output", totalOutput))

	return &TokenBudgetDecision{
		ShouldContinue:    true,
		Reason:            "budget headroom available",
		ContinuationCount: tracker.ContinuationCount,
	}
}
