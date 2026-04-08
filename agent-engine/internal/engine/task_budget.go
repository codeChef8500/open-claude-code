package engine

import (
	"log/slog"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Task budget & Feature flags runtime.
// Aligned with claude-code-main query.ts taskBudgetRemaining and featureFlags.
// ────────────────────────────────────────────────────────────────────────────

// TaskBudgetTracker tracks remaining dollars, tokens, and time for a task session.
// Aligned with claude-code-main TaskBudgetTracker.
type TaskBudgetTracker struct {
	mu sync.Mutex

	// MaxBudgetUSD is the maximum dollar spend for this task (0 = unlimited).
	MaxBudgetUSD float64
	// SpentUSD is the cumulative spend.
	SpentUSD float64

	// MaxTokens is the max total tokens (input+output) for this task (0 = unlimited).
	MaxTokens int
	// UsedTokens is the cumulative tokens consumed.
	UsedTokens int

	// MaxDuration is the maximum wall-clock time for this task (0 = unlimited).
	MaxDuration time.Duration
	// StartedAt is when the task started.
	StartedAt time.Time
}

// NewTaskBudgetTracker creates a task budget tracker from config.
func NewTaskBudgetTracker(maxUSD float64, maxTokens int, maxDuration time.Duration) *TaskBudgetTracker {
	return &TaskBudgetTracker{
		MaxBudgetUSD: maxUSD,
		MaxTokens:    maxTokens,
		MaxDuration:  maxDuration,
		StartedAt:    time.Now(),
	}
}

// Record adds usage from a single API call.
func (b *TaskBudgetTracker) Record(costUSD float64, tokens int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.SpentUSD += costUSD
	b.UsedTokens += tokens
}

// RemainingUSD returns the remaining dollar budget (or -1 if unlimited).
func (b *TaskBudgetTracker) RemainingUSD() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.MaxBudgetUSD <= 0 {
		return -1
	}
	rem := b.MaxBudgetUSD - b.SpentUSD
	if rem < 0 {
		rem = 0
	}
	return rem
}

// RemainingTokens returns the remaining token budget (or -1 if unlimited).
func (b *TaskBudgetTracker) RemainingTokens() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.MaxTokens <= 0 {
		return -1
	}
	rem := b.MaxTokens - b.UsedTokens
	if rem < 0 {
		rem = 0
	}
	return rem
}

// RemainingDuration returns the remaining time budget (or -1 if unlimited).
func (b *TaskBudgetTracker) RemainingDuration() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.MaxDuration <= 0 {
		return -1
	}
	elapsed := time.Since(b.StartedAt)
	rem := b.MaxDuration - elapsed
	if rem < 0 {
		rem = 0
	}
	return rem
}

// IsExhausted returns true if any budget dimension is exhausted.
func (b *TaskBudgetTracker) IsExhausted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.MaxBudgetUSD > 0 && b.SpentUSD >= b.MaxBudgetUSD {
		slog.Info("task_budget: dollar budget exhausted",
			slog.Float64("spent", b.SpentUSD),
			slog.Float64("max", b.MaxBudgetUSD))
		return true
	}
	if b.MaxTokens > 0 && b.UsedTokens >= b.MaxTokens {
		slog.Info("task_budget: token budget exhausted",
			slog.Int("used", b.UsedTokens),
			slog.Int("max", b.MaxTokens))
		return true
	}
	if b.MaxDuration > 0 {
		elapsed := time.Since(b.StartedAt)
		if elapsed >= b.MaxDuration {
			slog.Info("task_budget: time budget exhausted",
				slog.Duration("elapsed", elapsed),
				slog.Duration("max", b.MaxDuration))
			return true
		}
	}
	return false
}

// ExhaustionReason returns a human-readable reason if the budget is exhausted.
func (b *TaskBudgetTracker) ExhaustionReason() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.MaxBudgetUSD > 0 && b.SpentUSD >= b.MaxBudgetUSD {
		return "dollar budget exhausted"
	}
	if b.MaxTokens > 0 && b.UsedTokens >= b.MaxTokens {
		return "token budget exhausted"
	}
	if b.MaxDuration > 0 && time.Since(b.StartedAt) >= b.MaxDuration {
		return "time budget exhausted"
	}
	return ""
}

// Snapshot returns a read-only summary.
func (b *TaskBudgetTracker) Snapshot() TaskBudgetSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	return TaskBudgetSnapshot{
		MaxBudgetUSD: b.MaxBudgetUSD,
		SpentUSD:     b.SpentUSD,
		MaxTokens:    b.MaxTokens,
		UsedTokens:   b.UsedTokens,
		MaxDuration:  b.MaxDuration,
		Elapsed:      time.Since(b.StartedAt),
	}
}

// TaskBudgetSnapshot is a read-only copy of task budget state.
type TaskBudgetSnapshot struct {
	MaxBudgetUSD float64
	SpentUSD     float64
	MaxTokens    int
	UsedTokens   int
	MaxDuration  time.Duration
	Elapsed      time.Duration
}

// ── Feature flag gate helpers ────────────────────────────────────────────

// GateCheck evaluates a feature gate condition and returns the result.
// This is a convenience for queryloop-level checks.
func GateCheck(gates QueryGates, check func(QueryGates) bool) bool {
	return check(gates)
}

// CommonGateChecks provides pre-built gate check functions.
var CommonGateChecks = struct {
	EmitToolUseSummaries   func(QueryGates) bool
	FastModeEnabled        func(QueryGates) bool
	StreamingToolExecution func(QueryGates) bool
}{
	EmitToolUseSummaries: func(g QueryGates) bool {
		return g.EmitToolUseSummaries
	},
	FastModeEnabled: func(g QueryGates) bool {
		return g.FastModeEnabled
	},
	StreamingToolExecution: func(g QueryGates) bool {
		return g.StreamingToolExecution
	},
}
