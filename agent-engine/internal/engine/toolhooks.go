package engine

import (
	"context"
	"encoding/json"
	"log/slog"
)

// HookDecision is the outcome of a pre-tool hook.
type HookDecision struct {
	// Block is true when the hook vetoes the tool call entirely.
	Block bool
	// Reason is surfaced to the model as the tool-result error message.
	Reason string
	// ModifiedInput replaces the original input when non-nil.
	ModifiedInput json.RawMessage
}

// PreToolHook is a function invoked before every tool call.
// Returning a non-nil decision with Block=true prevents execution.
type PreToolHook func(ctx context.Context, toolName string, input json.RawMessage) (*HookDecision, error)

// PostToolHook is a function invoked after every tool call.
type PostToolHook func(ctx context.Context, toolName string, input json.RawMessage, result *ToolExecResult)

// ToolHookChain manages ordered lists of pre- and post-tool hooks and
// dispatches them around each tool execution.
type ToolHookChain struct {
	preHooks  []PreToolHook
	postHooks []PostToolHook
}

// NewToolHookChain creates an empty ToolHookChain.
func NewToolHookChain() *ToolHookChain { return &ToolHookChain{} }

// RegisterPre appends a pre-tool hook.
func (c *ToolHookChain) RegisterPre(h PreToolHook) {
	c.preHooks = append(c.preHooks, h)
}

// RegisterPost appends a post-tool hook.
func (c *ToolHookChain) RegisterPost(h PostToolHook) {
	c.postHooks = append(c.postHooks, h)
}

// RunPre fires all pre-tool hooks in registration order.  The first hook that
// returns Block=true short-circuits and the remaining hooks are skipped.
// If a hook modifies the input the modified value is returned for use in the
// actual tool call.
func (c *ToolHookChain) RunPre(ctx context.Context, toolName string, input json.RawMessage) (*HookDecision, json.RawMessage) {
	current := input
	for _, h := range c.preHooks {
		decision, err := h(ctx, toolName, current)
		if err != nil {
			slog.Warn("pre-tool hook error",
				slog.String("tool", toolName),
				slog.Any("err", err))
			continue
		}
		if decision == nil {
			continue
		}
		if decision.ModifiedInput != nil {
			current = decision.ModifiedInput
		}
		if decision.Block {
			return decision, current
		}
	}
	return nil, current
}

// RunPost fires all post-tool hooks in registration order.  Errors are logged
// and do not abort remaining hooks.
func (c *ToolHookChain) RunPost(ctx context.Context, toolName string, input json.RawMessage, result *ToolExecResult) {
	for _, h := range c.postHooks {
		h(ctx, toolName, input, result)
	}
}

// ToolHookExecutor wraps ToolExecutor with a ToolHookChain so every tool call
// automatically goes through the registered hook pipeline.
type ToolHookExecutor struct {
	inner *ToolExecutor
	chain *ToolHookChain
}

// NewToolHookExecutor creates a hook-aware executor.
func NewToolHookExecutor(inner *ToolExecutor, chain *ToolHookChain) *ToolHookExecutor {
	if chain == nil {
		chain = NewToolHookChain()
	}
	return &ToolHookExecutor{inner: inner, chain: chain}
}

// Execute fires pre-hooks, optionally blocks, then delegates to the inner
// executor, then fires post-hooks.
func (e *ToolHookExecutor) Execute(ctx context.Context, req *ToolExecRequest) *ToolExecResult {
	// ── Pre-hooks ─────────────────────────────────────────────────────────
	decision, modifiedInput := e.chain.RunPre(ctx, req.Tool.Name(), req.Input)
	if decision != nil && decision.Block {
		// Build a synthetic denied result.
		reason := decision.Reason
		if reason == "" {
			reason = "blocked by pre-tool hook"
		}
		result := &ToolExecResult{
			ToolUseID:     req.ToolUseID,
			ToolName:      req.Tool.Name(),
			IsError:       true,
			ErrorCategory: ToolExecErrPermission,
			Blocks: []*ContentBlock{{
				Type:    ContentTypeText,
				Text:    reason,
				IsError: true,
			}},
		}
		e.chain.RunPost(ctx, req.Tool.Name(), req.Input, result)
		return result
	}
	req.Input = modifiedInput

	// ── Delegate execution ────────────────────────────────────────────────
	result := e.inner.Execute(ctx, req)

	// ── Post-hooks ────────────────────────────────────────────────────────
	e.chain.RunPost(ctx, req.Tool.Name(), req.Input, result)
	return result
}
