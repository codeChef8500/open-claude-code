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
// Write tools (Bash, Edit, Write …) should override and return true.
func (b *BaseTool) IsDestructive() bool { return false }

// InterruptBehavior returns InterruptBehaviorNone by default
// (the tool is allowed to complete normally when the loop is cancelled).
func (b *BaseTool) InterruptBehavior() engine.InterruptBehavior {
	return engine.InterruptBehaviorNone
}

// IsSearchOrRead returns false by default.
// Read/search tools (Read, Grep, Glob …) should override and return true.
func (b *BaseTool) IsSearchOrRead() bool { return false }

// GetPath extracts no filesystem path from the input by default.
func (b *BaseTool) GetPath(_ json.RawMessage) string { return "" }

// ShouldDefer returns false by default (tool runs immediately in plan mode).
func (b *BaseTool) ShouldDefer() bool { return false }

// AlwaysLoad returns false by default; the tool is included only when active.
func (b *BaseTool) AlwaysLoad() bool { return false }

// SearchHint returns an empty hint by default.
func (b *BaseTool) SearchHint() string { return "" }

// GetActivityDescription returns an empty description by default.
func (b *BaseTool) GetActivityDescription(_ json.RawMessage) string { return "" }

// GetToolUseSummary returns an empty summary by default.
func (b *BaseTool) GetToolUseSummary(_ json.RawMessage) string { return "" }

// IsTransparentWrapper returns false by default.
func (b *BaseTool) IsTransparentWrapper() bool { return false }

// OutputSchema returns nil by default (no structured output schema).
func (b *BaseTool) OutputSchema() json.RawMessage { return nil }
