package provider

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// RateLimiterConfig configures the token-bucket rate limiter.
type RateLimiterConfig struct {
	// RequestsPerMinute is the max number of API calls per minute.
	RequestsPerMinute int
	// BurstSize allows a short burst above the steady-state rate.
	BurstSize int
}

// DefaultRateLimiterConfig returns sensible defaults for Anthropic API limits.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerMinute: 50,
		BurstSize:         5,
	}
}

// RateLimiter wraps a ModelCaller and enforces request rate limits using a
// token-bucket algorithm.
type RateLimiter struct {
	inner  engine.ModelCaller
	config RateLimiterConfig

	mu       sync.Mutex
	tokens   int
	maxTokens int
	lastFill time.Time
	fillRate float64 // tokens per second
}

// NewRateLimiter wraps inner with a rate limiter.
func NewRateLimiter(inner engine.ModelCaller, cfg RateLimiterConfig) *RateLimiter {
	if cfg.RequestsPerMinute <= 0 {
		cfg.RequestsPerMinute = 50
	}
	if cfg.BurstSize <= 0 {
		cfg.BurstSize = 5
	}
	maxTokens := cfg.BurstSize
	return &RateLimiter{
		inner:     inner,
		config:    cfg,
		tokens:    maxTokens,
		maxTokens: maxTokens,
		lastFill:  time.Now(),
		fillRate:  float64(cfg.RequestsPerMinute) / 60.0,
	}
}

// CallModel acquires a rate-limit token before forwarding to inner.
func (rl *RateLimiter) CallModel(ctx context.Context, params engine.CallParams) (<-chan *engine.StreamEvent, error) {
	if err := rl.acquire(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}
	return rl.inner.CallModel(ctx, params)
}

func (rl *RateLimiter) acquire(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refill()
		if rl.tokens > 0 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}
		// Calculate wait time until next token.
		wait := time.Duration(float64(time.Second) / rl.fillRate)
		rl.mu.Unlock()

		slog.Debug("rate limiter: waiting for token", slog.Duration("wait", wait))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			// Loop back and try again.
		}
	}
}

func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastFill).Seconds()
	newTokens := int(elapsed * rl.fillRate)
	if newTokens > 0 {
		rl.tokens += newTokens
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastFill = now
	}
}

// Available returns the current number of available tokens.
func (rl *RateLimiter) Available() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	return rl.tokens
}
