package bash

import (
	"context"
	"fmt"
	"time"
)

const (
	// DefaultTimeoutMs is the default bash execution timeout.
	DefaultTimeoutMs = 120_000 // 2 minutes

	// MaxTimeoutMs is the hard ceiling for any bash command.
	MaxTimeoutMs = 600_000 // 10 minutes

	// LongRunningThresholdMs is the threshold above which a command is
	// considered long-running and a warning is emitted.
	LongRunningThresholdMs = 30_000
)

// ResolveTimeout returns the effective timeout for a bash command.
// The caller-supplied value is clamped to [1s, MaxTimeoutMs].
// A value of 0 means use the default.
func ResolveTimeout(requestedMs int) time.Duration {
	if requestedMs <= 0 {
		return time.Duration(DefaultTimeoutMs) * time.Millisecond
	}
	if requestedMs > MaxTimeoutMs {
		requestedMs = MaxTimeoutMs
	}
	return time.Duration(requestedMs) * time.Millisecond
}

// WithTimeout wraps ctx with a deadline derived from timeoutMs.
// Returns the derived context and its cancel function.
func WithTimeout(ctx context.Context, timeoutMs int) (context.Context, context.CancelFunc) {
	d := ResolveTimeout(timeoutMs)
	return context.WithTimeout(ctx, d)
}

// TimeoutError is returned when a bash command exceeds its deadline.
type TimeoutError struct {
	Command   string
	TimeoutMs int
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("bash command timed out after %dms: %s", e.TimeoutMs, truncateCmd(e.Command, 80))
}

// IsTimeout reports whether err represents a bash timeout.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*TimeoutError)
	return ok
}

func truncateCmd(cmd string, max int) string {
	if len(cmd) <= max {
		return cmd
	}
	return cmd[:max] + "..."
}
