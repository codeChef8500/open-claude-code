package provider

import (
	"encoding/json"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// TextBlock creates a simple text ContentBlock.
func TextBlock(text string) *engine.ContentBlock {
	return &engine.ContentBlock{Type: engine.ContentTypeText, Text: text}
}

// ToolUseBlock creates a tool-use ContentBlock.
func ToolUseBlock(id, name string, input interface{}) *engine.ContentBlock {
	return &engine.ContentBlock{
		Type:      engine.ContentTypeToolUse,
		ToolUseID: id,
		ToolName:  name,
		Input:     input,
	}
}

// ToolResultBlock creates a tool-result ContentBlock.
func ToolResultBlock(toolUseID string, content []*engine.ContentBlock, isError bool) *engine.ContentBlock {
	return &engine.ContentBlock{
		Type:      engine.ContentTypeToolResult,
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}

// ThinkingBlock creates a thinking ContentBlock.
func ThinkingBlock(thinking, signature string) *engine.ContentBlock {
	return &engine.ContentBlock{
		Type:      engine.ContentTypeThinking,
		Thinking:  thinking,
		Signature: signature,
	}
}

// ParseToolInput attempts to unmarshal raw JSON bytes into a map.
// Returns nil on failure.
func ParseToolInput(raw json.RawMessage) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// MarshalInput converts any input value to a JSON raw message.
func MarshalInput(input interface{}) (json.RawMessage, error) {
	return json.Marshal(input)
}

// UserMessage is a convenience constructor for a user Message.
func UserMessage(text string) *engine.Message {
	return &engine.Message{
		Role:    engine.RoleUser,
		Content: []*engine.ContentBlock{TextBlock(text)},
	}
}

// AssistantTextMessage is a convenience constructor for an assistant text Message.
func AssistantTextMessage(text string) *engine.Message {
	return &engine.Message{
		Role:    engine.RoleAssistant,
		Content: []*engine.ContentBlock{TextBlock(text)},
	}
}
