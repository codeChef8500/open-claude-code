package service

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

// Pricing holds per-token pricing in USD for a model tier.
type Pricing struct {
	InputPerMToken        float64 // USD per million input tokens
	OutputPerMToken       float64 // USD per million output tokens
	CacheWritePerMToken   float64 // USD per million cache-write tokens
	CacheReadPerMToken    float64 // USD per million cache-read tokens
}

// Known model pricing (approximate, subject to change).
var ModelPricing = map[string]Pricing{
	"claude-opus-4-5":        {15.0, 75.0, 18.75, 1.50},
	"claude-sonnet-4-5":      {3.0, 15.0, 3.75, 0.30},
	"claude-haiku-4-5":       {0.25, 1.25, 0.30, 0.03},
	"claude-sonnet-4-20250514": {3.0, 15.0, 3.75, 0.30},
	"gpt-4o":                 {5.0, 15.0, 0, 0},
	"gpt-4o-mini":            {0.15, 0.60, 0, 0},
}

// DefaultPricing is used when the model is not in ModelPricing.
var DefaultPricing = Pricing{InputPerMToken: 3.0, OutputPerMToken: 15.0}

// CalcCost computes the USD cost of an API call given token counts.
func CalcCost(model string, inputTokens, outputTokens, cacheWriteTokens, cacheReadTokens int) float64 {
	p, ok := ModelPricing[model]
	if !ok {
		p = DefaultPricing
	}
	const M = 1_000_000.0
	return float64(inputTokens)/M*p.InputPerMToken +
		float64(outputTokens)/M*p.OutputPerMToken +
		float64(cacheWriteTokens)/M*p.CacheWritePerMToken +
		float64(cacheReadTokens)/M*p.CacheReadPerMToken
}

// CostTracker accumulates session cost in a thread-safe manner.
type CostTracker struct {
	total uint64 // stored as bits of float64
	limit float64
}

// NewCostTracker creates a CostTracker with an optional budget limit (0 = unlimited).
func NewCostTracker(limitUSD float64) *CostTracker {
	return &CostTracker{limit: limitUSD}
}

// Add adds delta USD to the running total.
func (ct *CostTracker) Add(delta float64) {
	for {
		old := atomic.LoadUint64(&ct.total)
		oldF := *(*float64)(unsafe.Pointer(&old))
		newF := oldF + delta
		newBits := *(*uint64)(unsafe.Pointer(&newF))
		if atomic.CompareAndSwapUint64(&ct.total, old, newBits) {
			return
		}
	}
}

// Total returns the current accumulated cost in USD.
func (ct *CostTracker) Total() float64 {
	bits := atomic.LoadUint64(&ct.total)
	return *(*float64)(unsafe.Pointer(&bits))
}

// ExceedsLimit reports whether the budget limit has been reached.
func (ct *CostTracker) ExceedsLimit() bool {
	return ct.limit > 0 && ct.Total() >= ct.limit
}

// FormatUSD formats a dollar amount as "$0.001234".
func FormatUSD(amount float64) string {
	return fmt.Sprintf("$%.6f", amount)
}
