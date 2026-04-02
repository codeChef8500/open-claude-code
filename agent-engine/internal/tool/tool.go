package tool

import (
	"context"
	"encoding/json"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// Tool is an alias for engine.Tool. The authoritative definition lives in the
// engine package to avoid the engine ↔ tool import cycle.
type Tool = engine.Tool

// UseContext is an alias for engine.UseContext.
type UseContext = engine.UseContext

// Result is a convenience helper to build a successful text result.
func Result(text string) []*engine.ContentBlock {
	return []*engine.ContentBlock{
		{Type: engine.ContentTypeText, Text: text},
	}
}

// ErrorResult builds an error result block.
func ErrorResult(msg string) []*engine.ContentBlock {
	return []*engine.ContentBlock{
		{Type: engine.ContentTypeText, Text: msg, IsError: true},
	}
}

// SendResult sends all content blocks to ch and closes it.
func SendResult(ch chan<- *engine.ContentBlock, blocks []*engine.ContentBlock) {
	for _, b := range blocks {
		ch <- b
	}
}

// BaseInputSchema is a helper that generates a simple JSON Schema from a
// Go struct using reflection. Tools can embed it or provide their own.
func BaseInputSchema(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

// BaseTool provides sensible default implementations for the optional Tool
// interface methods introduced in Phase 1.  Embed *BaseTool (or BaseTool) in
// any tool struct to satisfy the full engine.Tool interface without writing
// boilerplate for every method.
//
//	type MyTool struct {
//	    tool.BaseTool
//	    // ... fields
//	}
type BaseTool struct{}

// ValidateInput performs no structural validation by default (returns nil).
func (b *BaseTool) ValidateInput(_ context.Context, _ json.RawMessage) error { return nil }

// Aliases returns no alternate names by default.
func (b *BaseTool) Aliases() []string { return nil }

// IsDestructive returns false by default.
func (b *BaseTool) IsDestructive(_ json.RawMessage) bool { return false }

// IsReadOnly returns false by default.
func (b *BaseTool) IsReadOnly(_ json.RawMessage) bool { return false }

// IsConcurrencySafe returns false by default (assume not safe).
func (b *BaseTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

// InterruptBehavior returns InterruptBehaviorNone by default.
func (b *BaseTool) InterruptBehavior() engine.InterruptBehavior {
	return engine.InterruptBehaviorNone
}

// IsSearchOrRead returns all-false by default.
func (b *BaseTool) IsSearchOrRead(_ json.RawMessage) engine.SearchOrReadInfo {
	return engine.SearchOrReadInfo{}
}

// GetPath extracts no filesystem path from the input by default.
func (b *BaseTool) GetPath(_ json.RawMessage) string { return "" }

// ShouldDefer returns false by default.
func (b *BaseTool) ShouldDefer() bool { return false }

// AlwaysLoad returns false by default.
func (b *BaseTool) AlwaysLoad() bool { return false }

// SearchHint returns an empty hint by default.
func (b *BaseTool) SearchHint() string { return "" }

// GetActivityDescription returns an empty description by default.
func (b *BaseTool) GetActivityDescription(_ json.RawMessage) string { return "" }

// GetToolUseSummary returns an empty summary by default.
func (b *BaseTool) GetToolUseSummary(_ json.RawMessage) string { return "" }

// IsTransparentWrapper returns false by default.
func (b *BaseTool) IsTransparentWrapper() bool { return false }

// OutputSchema returns nil by default.
func (b *BaseTool) OutputSchema() json.RawMessage { return nil }

// IsMCP returns false by default.
func (b *BaseTool) IsMCP() bool { return false }

// IsLSP returns false by default.
func (b *BaseTool) IsLSP() bool { return false }

// IsOpenWorld returns false by default.
func (b *BaseTool) IsOpenWorld(_ json.RawMessage) bool { return false }

// RequiresUserInteraction returns false by default.
func (b *BaseTool) RequiresUserInteraction() bool { return false }

// Strict returns false by default.
func (b *BaseTool) Strict() bool { return false }

// ToAutoClassifierInput returns "" by default (skip classifier).
func (b *BaseTool) ToAutoClassifierInput(_ json.RawMessage) string { return "" }

// InputsEquivalent returns false by default (assume inputs are different).
func (b *BaseTool) InputsEquivalent(_, _ json.RawMessage) bool { return false }

// PreparePermissionMatcher returns nil by default.
func (b *BaseTool) PreparePermissionMatcher(_ json.RawMessage) func(string) bool { return nil }

// BackfillObservableInput does nothing by default.
func (b *BaseTool) BackfillObservableInput(_ map[string]interface{}) {}

// MapToolResultToBlockParam returns a simple text content block by default.
func (b *BaseTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text := ""
	if s, ok := content.(string); ok {
		text = s
	}
	return &engine.ContentBlock{
		Type:      engine.ContentTypeToolResult,
		ToolUseID: toolUseID,
		Text:      text,
	}
}

// IsResultTruncated returns false by default.
func (b *BaseTool) IsResultTruncated(_ interface{}) bool { return false }

// ContextModifier returns nil by default (no context modification).
func (b *BaseTool) ContextModifier() func(*engine.UseContext) *engine.UseContext { return nil }

// MCPInfo returns nil by default (not an MCP tool).
func (b *BaseTool) MCPInfo() *engine.MCPToolInfo { return nil }
