package provider

import (
	"context"
	"log/slog"
	"math/rand"
	"time"
)

// RetryConfig controls the exponential-backoff retry behaviour.
type RetryConfig struct {
	// MaxAttempts is the total number of tries (including the first).  Default 4.
	MaxAttempts int
	// BaseDelay is the initial back-off interval.  Default 1 s.
	BaseDelay time.Duration
	// MaxDelay caps the back-off interval.  Default 60 s.
	MaxDelay time.Duration
	// JitterFraction adds ±JitterFraction*delay random jitter.  Default 0.25.
	JitterFraction float64
}

// DefaultRetryConfig returns a sensible production retry config.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:    4,
		BaseDelay:      time.Second,
		MaxDelay:       60 * time.Second,
		JitterFraction: 0.25,
	}
}

// WithRetry calls fn up to cfg.MaxAttempts times, backing off between attempts.
// It stops immediately on non-retryable errors.
// If a server-supplied Retry-After value is available it is honoured.
func WithRetry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 4
	}
	if cfg.BaseDelay == 0 {
		cfg.BaseDelay = time.Second
	}
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = 60 * time.Second
	}
	if cfg.JitterFraction == 0 {
		cfg.JitterFraction = 0.25
	}

	var zero T
	var lastErr error
	delay := cfg.BaseDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		val, err := fn()
		if err == nil {
			return val, nil
		}
		lastErr = err

		apiErr := CategorizeRetryableAPIError(err)
		if apiErr == nil {
			// Non-retryable: give up immediately.
			return zero, err
		}

		if attempt == cfg.MaxAttempts {
			break
		}

		// Determine sleep duration.
		sleep := delay
		if apiErr.RetryAfterSecs > 0 {
			sleep = time.Duration(apiErr.RetryAfterSecs) * time.Second
		}
		sleep = addJitter(sleep, cfg.JitterFraction)
		if sleep > cfg.MaxDelay {
			sleep = cfg.MaxDelay
		}

		slog.Info("provider: retrying after error",
			slog.Int("attempt", attempt),
			slog.String("category", string(apiErr.Category)),
			slog.Duration("backoff", sleep))

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(sleep):
		}

		// Exponential back-off.
		delay *= 2
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}
	return zero, lastErr
}

// addJitter multiplies d by a random factor in [1-f, 1+f].
func addJitter(d time.Duration, f float64) time.Duration {
	if f <= 0 {
		return d
	}
	delta := float64(d) * f
	jitter := (rand.Float64()*2 - 1) * delta //nolint:gosec
	return time.Duration(float64(d) + jitter)
}
