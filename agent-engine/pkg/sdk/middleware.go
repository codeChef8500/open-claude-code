package sdk

import (
	"context"
	"log/slog"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// Middleware intercepts engine stream events, allowing logging, metrics,
// transformations, or filtering of the event stream.
type Middleware func(next EventHandler) EventHandler

// EventHandler processes a stream event and returns whether to continue.
type EventHandler func(ctx context.Context, ev *engine.StreamEvent) bool

// MiddlewareChain composes multiple middlewares into a single middleware.
func MiddlewareChain(mws ...Middleware) Middleware {
	return func(next EventHandler) EventHandler {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

// ── Built-in Middlewares ────────────────────────────────────────────────────

// LoggingMiddleware logs each stream event at debug level.
func LoggingMiddleware(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, ev *engine.StreamEvent) bool {
			if ev != nil {
				logger.Debug("sdk: stream event",
					slog.String("type", string(ev.Type)),
					slog.Int("text_len", len(ev.Text)))
			}
			return next(ctx, ev)
		}
	}
}

// MetricsMiddleware tracks event counts and token usage.
func MetricsMiddleware(collector *MetricsCollector) Middleware {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, ev *engine.StreamEvent) bool {
			if ev != nil {
				collector.RecordEvent(ev)
			}
			return next(ctx, ev)
		}
	}
}

// TimeoutMiddleware adds a per-event processing timeout.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, ev *engine.StreamEvent) bool {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return next(ctx, ev)
		}
	}
}

// FilterMiddleware drops events that don't match the predicate.
func FilterMiddleware(keep func(*engine.StreamEvent) bool) Middleware {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, ev *engine.StreamEvent) bool {
			if ev != nil && !keep(ev) {
				return true // skip but continue
			}
			return next(ctx, ev)
		}
	}
}

// ErrorCallbackMiddleware calls fn on every error event.
func ErrorCallbackMiddleware(fn func(error string)) Middleware {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, ev *engine.StreamEvent) bool {
			if ev != nil && ev.Type == engine.EventError && ev.Error != "" {
				fn(ev.Error)
			}
			return next(ctx, ev)
		}
	}
}

// ── MetricsCollector ────────────────────────────────────────────────────────

// MetricsCollector accumulates stream event metrics.
type MetricsCollector struct {
	EventCount   int64
	TextEvents   int64
	ToolEvents   int64
	ErrorEvents  int64
	InputTokens  int
	OutputTokens int
}

// RecordEvent updates counters based on the event type.
func (mc *MetricsCollector) RecordEvent(ev *engine.StreamEvent) {
	mc.EventCount++
	switch ev.Type {
	case engine.EventTextDelta:
		mc.TextEvents++
	case engine.EventToolUse, engine.EventToolResult:
		mc.ToolEvents++
	case engine.EventError:
		mc.ErrorEvents++
	case engine.EventUsage:
		if ev.Usage != nil {
			mc.InputTokens = ev.Usage.InputTokens
			mc.OutputTokens = ev.Usage.OutputTokens
		}
	}
}

// Reset clears all counters.
func (mc *MetricsCollector) Reset() {
	mc.EventCount = 0
	mc.TextEvents = 0
	mc.ToolEvents = 0
	mc.ErrorEvents = 0
	mc.InputTokens = 0
	mc.OutputTokens = 0
}

// ── Middleware-aware submit ──────────────────────────────────────────────────

// SubmitWithMiddleware sends a message and processes events through the middleware chain.
func (e *Engine) SubmitWithMiddleware(ctx context.Context, text string, mw Middleware) (string, error) {
	eventCh := e.inner.SubmitMessage(ctx, engine.QueryParams{Text: text})

	var result string
	var lastErr string

	handler := mw(func(_ context.Context, ev *engine.StreamEvent) bool {
		if ev == nil {
			return true
		}
		switch ev.Type {
		case engine.EventTextDelta:
			result += ev.Text
		case engine.EventError:
			lastErr = ev.Error
		}
		return true
	})

	for ev := range eventCh {
		if !handler(ctx, ev) {
			break
		}
	}

	if lastErr != "" {
		return result, &sdkError{msg: lastErr}
	}
	return result, nil
}

type sdkError struct{ msg string }

func (e *sdkError) Error() string { return e.msg }
