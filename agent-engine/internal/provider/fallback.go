package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// FallbackProvider wraps a primary provider with one or more fallback
// providers. If the primary returns a fallback-eligible error (overloaded,
// server error), the next provider in the chain is tried.
type FallbackProvider struct {
	providers []Provider
	names     []string
}

// NewFallbackProvider creates a provider that tries each provider in order.
// At least one provider must be supplied.
func NewFallbackProvider(providers []Provider, names []string) (*FallbackProvider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("fallback provider: at least one provider required")
	}
	if len(names) == 0 {
		names = make([]string, len(providers))
		for i := range providers {
			names[i] = fmt.Sprintf("provider-%d", i)
		}
	}
	return &FallbackProvider{providers: providers, names: names}, nil
}

// CallModel tries providers in order, falling back on eligible errors.
func (fp *FallbackProvider) CallModel(ctx context.Context, params engine.CallParams) (<-chan *engine.StreamEvent, error) {
	var lastErr error
	for i, p := range fp.providers {
		ch, err := p.CallModel(ctx, params)
		if err == nil {
			return ch, nil
		}
		lastErr = err

		// Check if this error is fallback-eligible.
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.IsFallbackable() && i < len(fp.providers)-1 {
			slog.Warn("provider: falling back",
				slog.String("from", fp.names[i]),
				slog.String("to", fp.names[i+1]),
				slog.String("reason", string(apiErr.Category)))
			continue
		}

		// Non-fallbackable error — stop immediately.
		return nil, err
	}
	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// CostTracker extends UsageAccumulator with per-turn cost tracking.
type CostTracker struct {
	accumulator *UsageAccumulator
	turns       []TurnCost
}

// TurnCost records the cost of a single LLM turn.
type TurnCost struct {
	Model        string
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	CostUSD      float64
}

// NewCostTracker creates a new cost tracker.
func NewCostTracker() *CostTracker {
	return &CostTracker{
		accumulator: &UsageAccumulator{},
	}
}

// RecordTurn records usage for a single LLM turn.
func (ct *CostTracker) RecordTurn(model string, input, output, cacheWrite, cacheRead int) {
	ct.accumulator.Record(model, input, output, cacheWrite, cacheRead)

	var costUSD float64
	if p, ok := LookupPricing(model); ok {
		costUSD = p.Cost(input, output, cacheWrite, cacheRead)
	}

	ct.turns = append(ct.turns, TurnCost{
		Model:        model,
		InputTokens:  input,
		OutputTokens: output,
		CacheRead:    cacheRead,
		CacheWrite:   cacheWrite,
		CostUSD:      costUSD,
	})
}

// TotalCostUSD returns the accumulated total cost.
func (ct *CostTracker) TotalCostUSD() float64 {
	return ct.accumulator.TotalCostUSD()
}

// Snapshot returns the current usage snapshot.
func (ct *CostTracker) Snapshot() UsageSnapshot {
	return ct.accumulator.Snapshot()
}

// TurnCount returns the number of recorded turns.
func (ct *CostTracker) TurnCount() int {
	return len(ct.turns)
}

// LastTurnCost returns the cost of the most recent turn, or zero.
func (ct *CostTracker) LastTurnCost() TurnCost {
	if len(ct.turns) == 0 {
		return TurnCost{}
	}
	return ct.turns[len(ct.turns)-1]
}

// AllTurns returns all recorded turn costs.
func (ct *CostTracker) AllTurns() []TurnCost {
	result := make([]TurnCost, len(ct.turns))
	copy(result, ct.turns)
	return result
}

// CostSummary returns a formatted cost summary string.
func (ct *CostTracker) CostSummary() string {
	snap := ct.Snapshot()
	return fmt.Sprintf("Cost: %s | Turns: %d | Input: %d | Output: %d | Cache R/W: %d/%d",
		FormatCost(snap.TotalCostUSD),
		len(ct.turns),
		snap.InputTokens,
		snap.OutputTokens,
		snap.CacheReadTokens,
		snap.CacheWriteTokens,
	)
}
