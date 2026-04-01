package tool

import (
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
