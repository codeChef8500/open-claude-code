package plugin

import (
	"context"
	"log/slog"
)

// HookType enumerates the 6 lifecycle hooks that plugins can intercept.
type HookType string

const (
	HookPreToolUse       HookType = "pre_tool_use"
	HookPostToolUse      HookType = "post_tool_use"
	HookStop             HookType = "stop"
	HookUserPromptSubmit HookType = "user_prompt_submit"
	HookSessionStart     HookType = "session_start"
	HookNotification     HookType = "notification"
)

// HookPayload carries the data passed to a hook handler.
type HookPayload struct {
	Type      HookType
	ToolName  string
	ToolInput interface{}
	Result    string
	SessionID string
	Message   string
}

// HookResult is the optional response from a hook handler.
type HookResult struct {
	// Modified allows a hook to replace the original value (tool input / prompt).
	Modified interface{}
	// Block instructs the engine to prevent the action (pre_tool_use only).
	Block  bool
	Reason string
}

// HookHandler is a function that plugins register for a specific hook.
type HookHandler func(ctx context.Context, payload HookPayload) (*HookResult, error)

// HookEngine dispatches lifecycle hooks to all registered handlers.
type HookEngine struct {
	handlers map[HookType][]HookHandler
}

// NewHookEngine creates an empty HookEngine.
func NewHookEngine() *HookEngine {
	return &HookEngine{handlers: make(map[HookType][]HookHandler)}
}

// Register adds a handler for a specific hook type.
func (he *HookEngine) Register(hookType HookType, fn HookHandler) {
	he.handlers[hookType] = append(he.handlers[hookType], fn)
}

// Run executes all handlers registered for hookType in registration order.
// The first handler that returns Block=true short-circuits execution.
func (he *HookEngine) Run(ctx context.Context, payload HookPayload) (*HookResult, error) {
	handlers := he.handlers[payload.Type]
	for _, h := range handlers {
		result, err := h(ctx, payload)
		if err != nil {
			slog.Warn("hook handler error",
				slog.String("hook", string(payload.Type)),
				slog.Any("err", err))
			continue
		}
		if result != nil && result.Block {
			return result, nil
		}
	}
	return nil, nil
}

// RunPreToolUse fires the pre_tool_use hook and returns whether the tool call
// should be blocked.
func (he *HookEngine) RunPreToolUse(ctx context.Context, toolName string, input interface{}) (bool, string) {
	result, _ := he.Run(ctx, HookPayload{
		Type:      HookPreToolUse,
		ToolName:  toolName,
		ToolInput: input,
	})
	if result != nil && result.Block {
		return true, result.Reason
	}
	return false, ""
}

// RunPostToolUse fires the post_tool_use hook.
func (he *HookEngine) RunPostToolUse(ctx context.Context, toolName string, result string) {
	he.Run(ctx, HookPayload{ //nolint:errcheck
		Type:     HookPostToolUse,
		ToolName: toolName,
		Result:   result,
	})
}

// RunSessionStart fires the session_start hook.
func (he *HookEngine) RunSessionStart(ctx context.Context, sessionID string) {
	he.Run(ctx, HookPayload{ //nolint:errcheck
		Type:      HookSessionStart,
		SessionID: sessionID,
	})
}

// RunUserPromptSubmit fires the user_prompt_submit hook.
func (he *HookEngine) RunUserPromptSubmit(ctx context.Context, message string) {
	he.Run(ctx, HookPayload{ //nolint:errcheck
		Type:    HookUserPromptSubmit,
		Message: message,
	})
}

// RunNotification fires the notification hook.
func (he *HookEngine) RunNotification(ctx context.Context, message string) {
	he.Run(ctx, HookPayload{ //nolint:errcheck
		Type:    HookNotification,
		Message: message,
	})
}
