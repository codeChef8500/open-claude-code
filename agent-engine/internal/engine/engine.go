package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/state"
)

// Engine manages a single conversation session with an LLM.
// It is the top-level object callers interact with.
type Engine struct {
	cfg      EngineConfig
	caller ModelCaller
	tools    []Tool
	store    *state.Store
	session  *state.SessionState
}

// New creates and initialises an Engine from the given config.
func New(cfg EngineConfig, prov ModelCaller, tools []Tool) (*Engine, error) {
	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("EngineConfig.WorkDir must not be empty")
	}
	if cfg.SessionID == "" {
		cfg.SessionID = uuid.New().String()
	}

	store := state.NewStore()
	store.Set("model", cfg.Model)
	store.Set("verbose", cfg.Verbose)
	store.Set("auto_mode", cfg.AutoMode)

	sess := state.NewSessionState(cfg.SessionID, cfg.WorkDir)

	return &Engine{
		cfg:      cfg,
		caller: prov,
		tools:    tools,
		store:    store,
		session:  sess,
	}, nil
}

// SessionID returns the unique identifier of this session.
func (e *Engine) SessionID() string { return e.session.SessionID() }

// Store returns the mutable state store.
func (e *Engine) Store() *state.Store { return e.store }

// SubmitMessage sends a user message and returns a channel of StreamEvents.
// The channel is closed when the engine has finished processing (either
// naturally or due to context cancellation).
func (e *Engine) SubmitMessage(ctx context.Context, params QueryParams) <-chan *StreamEvent {
	ch := make(chan *StreamEvent, 128)
	go func() {
		defer close(ch)
		if err := runQueryLoop(ctx, e, params, ch); err != nil {
			if ctx.Err() == nil {
				ch <- &StreamEvent{
					Type:  EventError,
					Error: err.Error(),
				}
			}
		}
	}()
	return ch
}

// Close releases any resources held by the engine.
func (e *Engine) Close() error { return nil }

// useContext builds a UseContext for the current session.
func (e *Engine) useContext() *UseContext {
	return &UseContext{
		WorkDir:   e.cfg.WorkDir,
		SessionID: e.cfg.SessionID,
		AutoMode:  e.cfg.AutoMode,
	}
}

// enabledTools returns only the tools that are currently enabled.
func (e *Engine) enabledTools() []Tool {
	uctx := e.useContext()
	var enabled []Tool
	for _, t := range e.tools {
		if t.IsEnabled(uctx) {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// toolDefs converts enabled tools to ToolDefinition format.
func (e *Engine) toolDefs() []ToolDefinition {
	var defs []ToolDefinition
	for _, t := range e.enabledTools() {
		defs = append(defs, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return defs
}

// findTool looks up a tool by name.
func (e *Engine) findTool(name string) (Tool, bool) {
	for _, t := range e.tools {
		if t.Name() == name {
			return t, true
		}
	}
	return nil, false
}

// emitSystemMessage sends a non-LLM status update to the caller.
func emitSystemMessage(ch chan<- *StreamEvent, msg string) {
	select {
	case ch <- &StreamEvent{Type: EventSystemMessage, Text: msg}:
	default:
		slog.Debug("system message dropped (channel full)", slog.String("msg", msg))
	}
}

// computeCostUSD estimates the USD cost for the given usage stats.
// Prices are based on Claude Sonnet 4 list prices (update as needed).
func computeCostUSD(usage *UsageStats, model string) float64 {
	// Default to Sonnet pricing
	inputCPM := 3.0  // $ per million tokens
	outputCPM := 15.0

	microUSD := float64(usage.InputTokens)*inputCPM/1_000_000 +
		float64(usage.OutputTokens)*outputCPM/1_000_000 +
		float64(usage.CacheCreationInputTokens)*inputCPM*1.25/1_000_000 // cache write is 25% more

	_ = model // future: per-model pricing
	_ = time.Now()
	return microUSD
}
