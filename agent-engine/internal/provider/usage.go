package provider

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ModelPricing holds per-million-token prices for a model.
type ModelPricing struct {
	InputPerMillion       float64
	OutputPerMillion      float64
	CacheWritePerMillion  float64
	CacheReadPerMillion   float64
}

// Cost computes the USD cost for the given token counts.
func (p ModelPricing) Cost(inputTokens, outputTokens, cacheWrite, cacheRead int) float64 {
	return float64(inputTokens)/1_000_000*p.InputPerMillion +
		float64(outputTokens)/1_000_000*p.OutputPerMillion +
		float64(cacheWrite)/1_000_000*p.CacheWritePerMillion +
		float64(cacheRead)/1_000_000*p.CacheReadPerMillion
}

// knownPricing maps model prefixes to pricing info.
var knownPricing = map[string]ModelPricing{
	"claude-opus-4":         {InputPerMillion: 15.0, OutputPerMillion: 75.0, CacheWritePerMillion: 18.75, CacheReadPerMillion: 1.50},
	"claude-sonnet-4":       {InputPerMillion: 3.0, OutputPerMillion: 15.0, CacheWritePerMillion: 3.75, CacheReadPerMillion: 0.30},
	"claude-3-7-sonnet":     {InputPerMillion: 3.0, OutputPerMillion: 15.0, CacheWritePerMillion: 3.75, CacheReadPerMillion: 0.30},
	"claude-3-5-sonnet":     {InputPerMillion: 3.0, OutputPerMillion: 15.0, CacheWritePerMillion: 3.75, CacheReadPerMillion: 0.30},
	"claude-3-5-haiku":      {InputPerMillion: 0.80, OutputPerMillion: 4.0, CacheWritePerMillion: 1.0, CacheReadPerMillion: 0.08},
	"claude-3-haiku":        {InputPerMillion: 0.25, OutputPerMillion: 1.25, CacheWritePerMillion: 0.30, CacheReadPerMillion: 0.03},
	"claude-3-opus":         {InputPerMillion: 15.0, OutputPerMillion: 75.0, CacheWritePerMillion: 18.75, CacheReadPerMillion: 1.50},
}

// LookupPricing returns the pricing for a model by prefix match.
// Returns zero pricing and false if unknown.
func LookupPricing(model string) (ModelPricing, bool) {
	for prefix, p := range knownPricing {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return p, true
		}
	}
	return ModelPricing{}, false
}

// UsageAccumulator is a thread-safe accumulator for API usage across all
// calls in a session.
type UsageAccumulator struct {
	mu sync.Mutex

	InputTokens      int64
	OutputTokens     int64
	CacheWriteTokens int64
	CacheReadTokens  int64
	Requests         int64

	// totalMicroUSD is accumulated cost in micro-dollars for lock-free reads.
	totalMicroUSD atomic.Int64
}

// Record adds a single call's usage to the accumulator.
func (a *UsageAccumulator) Record(model string, input, output, cacheWrite, cacheRead int) {
	a.mu.Lock()
	a.InputTokens += int64(input)
	a.OutputTokens += int64(output)
	a.CacheWriteTokens += int64(cacheWrite)
	a.CacheReadTokens += int64(cacheRead)
	a.Requests++
	a.mu.Unlock()

	if p, ok := LookupPricing(model); ok {
		cost := p.Cost(input, output, cacheWrite, cacheRead)
		a.totalMicroUSD.Add(int64(cost * 1_000_000))
	}
}

// TotalCostUSD returns the accumulated cost across all recorded calls.
func (a *UsageAccumulator) TotalCostUSD() float64 {
	return float64(a.totalMicroUSD.Load()) / 1_000_000
}

// Snapshot returns a copy of the current usage totals.
func (a *UsageAccumulator) Snapshot() UsageSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return UsageSnapshot{
		InputTokens:      a.InputTokens,
		OutputTokens:     a.OutputTokens,
		CacheWriteTokens: a.CacheWriteTokens,
		CacheReadTokens:  a.CacheReadTokens,
		Requests:         a.Requests,
		TotalCostUSD:     float64(a.totalMicroUSD.Load()) / 1_000_000,
	}
}

// UsageSnapshot is a point-in-time copy of usage statistics.
type UsageSnapshot struct {
	InputTokens      int64
	OutputTokens     int64
	CacheWriteTokens int64
	CacheReadTokens  int64
	Requests         int64
	TotalCostUSD     float64
}

// FormatCost returns a human-readable cost string, e.g. "$0.0123".
func FormatCost(usd float64) string {
	if usd < 0.01 {
		return fmt.Sprintf("$%.4f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
}
