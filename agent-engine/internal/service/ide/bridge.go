package ide

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// Handler processes an IDERequest and returns an IDEResponse.
type Handler func(ctx context.Context, req *IDERequest) *IDEResponse

// Bridge is the bidirectional communication layer between the agent engine
// and the IDE extension.  It dispatches inbound IDE requests to registered
// method handlers and provides a Send channel for pushing events to the IDE.
type Bridge struct {
	mu       sync.RWMutex
	handlers map[string]Handler

	// Outbound is the channel the IDE reader goroutine consumes.
	// Buffer size is generous to prevent blocking the engine.
	Outbound chan *IDEResponse

	capabilities IDECapabilities
}

// NewBridge creates a Bridge with the given outbound buffer size.
func NewBridge(outboundBuf int) *Bridge {
	if outboundBuf <= 0 {
		outboundBuf = 64
	}
	b := &Bridge{
		handlers: make(map[string]Handler),
		Outbound: make(chan *IDEResponse, outboundBuf),
	}
	b.registerBuiltins()
	return b
}

// Register adds a method handler.  It is safe to call before the bridge starts.
func (b *Bridge) Register(method string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[method] = h
}

// Dispatch routes an inbound request to the appropriate handler.
// The response (if any) is returned directly; the caller may also write it to
// Outbound if the exchange is asynchronous.
func (b *Bridge) Dispatch(ctx context.Context, req *IDERequest) *IDEResponse {
	b.mu.RLock()
	h, ok := b.handlers[req.Method]
	b.mu.RUnlock()

	if !ok {
		return &IDEResponse{
			Error: &IDEError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
	return h(ctx, req)
}

// Send pushes an event response to the outbound channel (non-blocking).
// Returns false if the channel is full.
func (b *Bridge) Send(resp *IDEResponse) bool {
	select {
	case b.Outbound <- resp:
		return true
	default:
		slog.Warn("ide bridge: outbound buffer full, dropping event",
			slog.Any("resp", resp))
		return false
	}
}

// NotifyDiagnostics pushes a diagnostics update for a file URI.
func (b *Bridge) NotifyDiagnostics(uri string, diags []Diagnostic) {
	payload, _ := json.Marshal(map[string]interface{}{
		"uri":         uri,
		"diagnostics": diags,
	})
	b.Send(&IDEResponse{
		Result: json.RawMessage(payload),
	})
}

// Capabilities returns the negotiated IDE capabilities.
func (b *Bridge) Capabilities() IDECapabilities {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.capabilities
}

// ── Built-in method handlers ─────────────────────────────────────────────────

func (b *Bridge) registerBuiltins() {
	b.Register("initialize", b.handleInitialize)
	b.Register("workspace/didChangeWatchedFiles", b.handleFileChange)
	b.Register("ping", b.handlePing)
}

func (b *Bridge) handleInitialize(ctx context.Context, req *IDERequest) *IDEResponse {
	_ = ctx
	var caps IDECapabilities
	if params, ok := req.Params.(map[string]interface{}); ok {
		if name, ok := params["ide_name"].(string); ok {
			caps.IDEName = name
		}
		if ver, ok := params["ide_version"].(string); ok {
			caps.IDEVersion = ver
		}
		if v, ok := params["supports_inlay_hints"].(bool); ok {
			caps.SupportsInlayHints = v
		}
	}
	b.mu.Lock()
	b.capabilities = caps
	b.mu.Unlock()

	slog.Info("ide bridge: initialized",
		slog.String("ide", caps.IDEName),
		slog.String("version", caps.IDEVersion))

	return &IDEResponse{Result: map[string]string{"status": "ok"}}
}

func (b *Bridge) handleFileChange(_ context.Context, req *IDERequest) *IDEResponse {
	_ = req
	return &IDEResponse{Result: map[string]string{"status": "ok"}}
}

func (b *Bridge) handlePing(_ context.Context, _ *IDERequest) *IDEResponse {
	return &IDEResponse{Result: map[string]string{"pong": "true"}}
}
