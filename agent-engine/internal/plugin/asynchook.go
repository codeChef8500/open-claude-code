package plugin

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// AsyncHookEngine wraps HookEngine and dispatches all handlers in background
// goroutines so they never block the main query loop.  Callers can optionally
// wait for all in-flight hooks to finish before shutdown.
type AsyncHookEngine struct {
	inner   *HookEngine
	wg      sync.WaitGroup
	timeout time.Duration
}

// NewAsyncHookEngine wraps an existing HookEngine.  handlerTimeout controls
// how long a single async handler may run before its context is cancelled
// (0 = no timeout).
func NewAsyncHookEngine(inner *HookEngine, handlerTimeout time.Duration) *AsyncHookEngine {
	return &AsyncHookEngine{inner: inner, timeout: handlerTimeout}
}

// Register adds a handler for a specific hook type (delegates to inner engine).
func (a *AsyncHookEngine) Register(hookType HookType, fn HookHandler) {
	a.inner.Register(hookType, fn)
}

// FireAsync dispatches all handlers for hookType in separate goroutines.
// It never blocks and always returns immediately.  Results and errors are
// logged at Debug level.
func (a *AsyncHookEngine) FireAsync(parentCtx context.Context, payload HookPayload) {
	handlers := a.inner.handlers[payload.Type]
	for _, h := range handlers {
		h := h
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			ctx := parentCtx
			var cancel context.CancelFunc
			if a.timeout > 0 {
				ctx, cancel = context.WithTimeout(parentCtx, a.timeout)
				defer cancel()
			}
			result, err := h(ctx, payload)
			if err != nil {
				slog.Debug("async hook error",
					slog.String("hook", string(payload.Type)),
					slog.Any("err", err))
				return
			}
			if result != nil && result.Block {
				slog.Debug("async hook returned Block=true (ignored in async mode)",
					slog.String("hook", string(payload.Type)),
					slog.String("reason", result.Reason))
			}
		}()
	}
}

// Wait blocks until all in-flight async handlers have completed.
func (a *AsyncHookEngine) Wait() { a.wg.Wait() }

// RunPreToolUse fires pre_tool_use synchronously (blocking) because it may
// need to block a tool call before execution starts.
func (a *AsyncHookEngine) RunPreToolUse(ctx context.Context, toolName string, input interface{}) (bool, string) {
	return a.inner.RunPreToolUse(ctx, toolName, input)
}

// RunPostToolUse fires post_tool_use asynchronously.
func (a *AsyncHookEngine) RunPostToolUse(ctx context.Context, toolName, result string) {
	a.FireAsync(ctx, HookPayload{
		Type:     HookPostToolUse,
		ToolName: toolName,
		Result:   result,
	})
}

// RunSessionStart fires session_start asynchronously.
func (a *AsyncHookEngine) RunSessionStart(ctx context.Context, sessionID string) {
	a.FireAsync(ctx, HookPayload{
		Type:      HookSessionStart,
		SessionID: sessionID,
	})
}

// RunNotification fires notification asynchronously.
func (a *AsyncHookEngine) RunNotification(ctx context.Context, message string) {
	a.FireAsync(ctx, HookPayload{
		Type:    HookNotification,
		Message: message,
	})
}
