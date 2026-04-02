package hooks

import (
	"context"
	"sync"
	"time"
)

// HandlerFunc is a programmatic hook handler (alternative to external scripts).
type HandlerFunc func(ctx context.Context, input *HookInput) (*HookJSONOutput, error)

// RegisteredHandler is a programmatic hook registered at runtime.
type RegisteredHandler struct {
	Event   HookEvent
	Name    string
	Handler HandlerFunc
	Async   bool
}

// Registry allows registering programmatic hook handlers alongside
// file-based hook scripts. Programmatic hooks run in-process without
// subprocess overhead.
type Registry struct {
	mu       sync.RWMutex
	handlers map[HookEvent][]RegisteredHandler
}

// NewRegistry creates an empty hook registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[HookEvent][]RegisteredHandler),
	}
}

// Register adds a synchronous programmatic hook handler.
func (r *Registry) Register(event HookEvent, name string, fn HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[event] = append(r.handlers[event], RegisteredHandler{
		Event:   event,
		Name:    name,
		Handler: fn,
	})
}

// RegisterAsync adds an asynchronous programmatic hook handler.
func (r *Registry) RegisterAsync(event HookEvent, name string, fn HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[event] = append(r.handlers[event], RegisteredHandler{
		Event:   event,
		Name:    name,
		Handler: fn,
		Async:   true,
	})
}

// Unregister removes all handlers with the given name for the event.
func (r *Registry) Unregister(event HookEvent, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	handlers := r.handlers[event]
	filtered := handlers[:0]
	for _, h := range handlers {
		if h.Name != name {
			filtered = append(filtered, h)
		}
	}
	r.handlers[event] = filtered
}

// HasHandlers reports whether any programmatic handlers are registered for the event.
func (r *Registry) HasHandlers(event HookEvent) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers[event]) > 0
}

// RunSync executes all sync programmatic handlers for the event.
func (r *Registry) RunSync(ctx context.Context, event HookEvent, input *HookInput) SyncHookResponse {
	r.mu.RLock()
	handlers := r.handlers[event]
	r.mu.RUnlock()

	if len(handlers) == 0 {
		return SyncHookResponse{}
	}

	input.Event = event
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now()
	}

	var merged SyncHookResponse
	for _, h := range handlers {
		if h.Async {
			continue
		}
		out, err := h.Handler(ctx, input)
		if err != nil {
			merged.Error = err
			return merged
		}
		if out == nil {
			continue
		}
		resp := parseHookOutput(*out)
		mergeResponse(&merged, &resp)
		if resp.Decision == "block" {
			break
		}
	}
	return merged
}

// RunAsync fires all async programmatic handlers for the event.
func (r *Registry) RunAsync(event HookEvent, input *HookInput) {
	r.mu.RLock()
	handlers := r.handlers[event]
	r.mu.RUnlock()

	input.Event = event
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now()
	}

	for _, h := range handlers {
		if !h.Async {
			continue
		}
		go func(handler RegisteredHandler) {
			ctx, cancel := context.WithTimeout(context.Background(), defaultHookTimeout)
			defer cancel()
			_, _ = handler.Handler(ctx, input)
		}(h)
	}
}

func mergeResponse(dst, src *SyncHookResponse) {
	if src.Decision != "" {
		dst.Decision = src.Decision
	}
	if src.ShouldStop {
		dst.ShouldStop = true
		dst.StopReason = src.StopReason
	}
	if src.UpdatedInput != nil {
		dst.UpdatedInput = src.UpdatedInput
	}
	if src.AdditionalContext != "" {
		dst.AdditionalContext = src.AdditionalContext
	}
	if src.OutputOverride != nil {
		dst.OutputOverride = src.OutputOverride
	}
	if src.Passed != nil {
		dst.Passed = src.Passed
	}
	if src.FailureReason != "" {
		dst.FailureReason = src.FailureReason
	}
	if src.NewCustomInstructions != "" {
		dst.NewCustomInstructions = src.NewCustomInstructions
	}
}
