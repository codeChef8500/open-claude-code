package engine

import (
	"context"

	"github.com/wall-ai/agent-engine/internal/state"
)

// ────────────────────────────────────────────────────────────────────────────
// ToolUseContext — aggregates all context needed during tool execution.
// Aligned with claude-code-main Tool.ts ToolUseContext.
// ────────────────────────────────────────────────────────────────────────────

// ToolUseContext carries all state that tools and the query loop need during
// a single query invocation.  It is created at the start of each
// SubmitMessage call and threaded through the entire execution.
type ToolUseContext struct {
	// Options holds the immutable tool execution configuration.
	Options *ToolUseOptions

	// AbortCtx is the context that is cancelled when the user interrupts.
	AbortCtx context.Context
	// AbortCancel cancels AbortCtx (e.g. on user interrupt).
	AbortCancel context.CancelFunc

	// GetAppState returns the current application state as a generic map.
	// Uses Store.Get("app_state") under the hood.
	GetAppState func() *state.AppState
	// SetAppState atomically updates the application state.
	SetAppState func(fn func(*state.AppState) *state.AppState)

	// ReadFileState tracks file read cache state for dedup/unchanged detection.
	// Uses the existing FileStateCache from filecache.go.
	ReadFileState *FileStateCache

	// QueryTracking holds the query chain ID and depth for analytics.
	QueryTracking *QueryTracking

	// AgentID is non-empty when this context belongs to a sub-agent.
	AgentID string
	// AgentType is the type of the agent (e.g. "code", "research").
	AgentType string

	// AddNotification sends a notification to the UI layer.
	AddNotification func(text string)
	// AppendSystemMessage injects a system message into the conversation.
	AppendSystemMessage func(text string)

	// HandleElicitation is called when the model requests user input mid-turn.
	HandleElicitation func(ctx context.Context, toolUseID string, question string) (string, error)

	// ContentReplacementState tracks tool-result-budget disk references.
	// Uses the existing ContentReplacementState from content_replacement.go.
	ContentReplacementState *ContentReplacementState
}

// ToolUseOptions holds the immutable configuration for tool execution within
// a query.  Aligned with claude-code-main ToolUseContext.options.
type ToolUseOptions struct {
	// Tools is the full set of tools available for this query.
	Tools []Tool
	// MainLoopModel is the model name used for the main query loop.
	MainLoopModel string
	// ThinkingConfig holds extended thinking configuration.
	// Uses the existing ThinkingConfig from thinking.go.
	ThinkingConfig *ThinkingConfig
	// IsNonInteractiveSession is true for CI/CD or SDK mode (no user prompts).
	IsNonInteractiveSession bool
	// AppendSystemPrompt is extra text appended to the system prompt.
	AppendSystemPrompt string

	// AgentDefinitions holds the active agents and allowed agent types.
	AgentDefinitions *AgentDefinitions

	// RefreshTools is called between turns to pick up newly-connected MCP
	// servers.  Returns the updated tool list, or the same slice if unchanged.
	RefreshTools func() []Tool
}

// AgentDefinitions describes available agents and permission rules.
type AgentDefinitions struct {
	// ActiveAgents is the list of currently active agent definitions.
	ActiveAgents []AgentDefinition
	// AllowedAgentTypes restricts which agent types can be spawned.
	AllowedAgentTypes []string
}

// AgentDefinition describes a single agent type.
type AgentDefinition struct {
	// Name is the agent's display name.
	Name string
	// Type is the agent type identifier (e.g. "code", "research").
	Type string
	// Description is a human-readable description of what the agent does.
	Description string
}

// NewToolUseContext creates a ToolUseContext from the engine state.
// The AppState accessors use Store.Get/Set with the "app_state" key.
func NewToolUseContext(
	ctx context.Context,
	e *Engine,
	tools []Tool,
	model string,
	queryTracking *QueryTracking,
) *ToolUseContext {
	abortCtx, abortCancel := context.WithCancel(ctx)

	return &ToolUseContext{
		Options: &ToolUseOptions{
			Tools:                   tools,
			MainLoopModel:           model,
			IsNonInteractiveSession: e.cfg.NonInteractive,
			AppendSystemPrompt:      e.cfg.AppendSystemPrompt,
		},
		AbortCtx:    abortCtx,
		AbortCancel: abortCancel,
		GetAppState: func() *state.AppState {
			v := e.store.Get("app_state")
			if as, ok := v.(*state.AppState); ok {
				return as
			}
			// Return a default if not yet set.
			return &state.AppState{}
		},
		SetAppState: func(fn func(*state.AppState) *state.AppState) {
			v := e.store.Get("app_state")
			current, _ := v.(*state.AppState)
			if current == nil {
				current = &state.AppState{}
			}
			updated := fn(current)
			e.store.Set("app_state", updated)
		},
		ReadFileState:           NewFileStateCache(200),
		QueryTracking:           queryTracking,
		ContentReplacementState: NewContentReplacementState(0),
		AddNotification:         func(string) {}, // wired by SDK layer
		AppendSystemMessage:     func(string) {}, // wired by SDK layer
	}
}
