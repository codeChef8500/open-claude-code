package util

import (
	"context"
	"sync"
)

// CancelGroup manages a set of context cancel functions, allowing bulk
// cancellation. It is the Go equivalent of managing multiple AbortControllers.
type CancelGroup struct {
	mu      sync.Mutex
	cancels []context.CancelFunc
}

// Add registers a cancel function to the group.
func (g *CancelGroup) Add(cancel context.CancelFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cancels = append(g.cancels, cancel)
}

// CancelAll invokes every registered cancel function.
func (g *CancelGroup) CancelAll() {
	g.mu.Lock()
	cancels := make([]context.CancelFunc, len(g.cancels))
	copy(cancels, g.cancels)
	g.cancels = nil
	g.mu.Unlock()

	for _, c := range cancels {
		c()
	}
}

// WithCancel is a convenience wrapper around context.WithCancel that also
// registers the cancel function in the group.
func (g *CancelGroup) WithCancel(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	g.Add(cancel)
	return ctx, cancel
}

// IsAborted reports whether the error represents a deliberate cancellation
// (context.Canceled, context.DeadlineExceeded, or *AbortError).
func IsAborted(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled || err == context.DeadlineExceeded {
		return true
	}
	var abort *AbortError
	return As(err, &abort)
}

// As is a thin alias for errors.As for convenience within this package.
func As(err error, target interface{}) bool {
	// Using the standard library via the errors package would create a cycle;
	// the engine imports util, so we inline the type assertion.
	switch t := target.(type) {
	case **AbortError:
		if a, ok := err.(*AbortError); ok {
			*t = a
			return true
		}
	case **ShellError:
		if s, ok := err.(*ShellError); ok {
			*t = s
			return true
		}
	}
	return false
}
