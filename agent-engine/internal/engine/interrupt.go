package engine

import (
	"context"
	"sync"
)

// InterruptReason describes why a query loop was interrupted.
type InterruptReason string

const (
	InterruptUserCancel    InterruptReason = "user_cancel"
	InterruptTokenBudget   InterruptReason = "token_budget"
	InterruptCostLimit     InterruptReason = "cost_limit"
	InterruptToolDenied    InterruptReason = "tool_denied"
	InterruptMaxTurns      InterruptReason = "max_turns"
	InterruptStopHook      InterruptReason = "stop_hook"
	InterruptContextDone   InterruptReason = "context_done"
)

// InterruptSignal is a thread-safe one-shot interrupt for the query loop.
// Once fired it stays fired; subsequent fires are no-ops.
type InterruptSignal struct {
	mu     sync.Mutex
	fired  bool
	reason InterruptReason
	ch     chan struct{}
}

// NewInterruptSignal creates a fresh, unfired signal.
func NewInterruptSignal() *InterruptSignal {
	return &InterruptSignal{ch: make(chan struct{})}
}

// Fire triggers the interrupt with the given reason.
// Safe to call multiple times; only the first reason is recorded.
func (s *InterruptSignal) Fire(reason InterruptReason) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fired {
		return
	}
	s.fired = true
	s.reason = reason
	close(s.ch)
}

// Fired reports whether the signal has been triggered.
func (s *InterruptSignal) Fired() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fired
}

// Reason returns the interrupt reason, or "" if not yet fired.
func (s *InterruptSignal) Reason() InterruptReason {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

// Chan returns a channel that is closed when the signal fires.
// Useful for select statements.
func (s *InterruptSignal) Chan() <-chan struct{} {
	return s.ch
}

// Context returns a derived context that is cancelled when the signal fires.
func (s *InterruptSignal) Context(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-s.ch:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// InterruptError is returned by the query loop when interrupted.
type InterruptError struct {
	Reason InterruptReason
}

func (e *InterruptError) Error() string {
	return "query loop interrupted: " + string(e.Reason)
}

// IsInterrupt reports whether err is an InterruptError.
func IsInterrupt(err error) bool {
	_, ok := err.(*InterruptError)
	return ok
}
