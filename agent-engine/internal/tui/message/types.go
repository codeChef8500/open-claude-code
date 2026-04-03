package message

import "time"

// MessageType identifies the role of a message in the conversation.
type MessageType string

const (
	TypeUser      MessageType = "user"
	TypeAssistant MessageType = "assistant"
	TypeSystem    MessageType = "system"
	TypeError     MessageType = "error"
	TypeToolUse   MessageType = "tool_use"
	TypeToolResult MessageType = "tool_result"
	TypeThinking  MessageType = "thinking"
	TypeProgress  MessageType = "progress"
	TypeCompact   MessageType = "compact_boundary"
	TypeAttachment MessageType = "attachment"
)

// ContentBlockType identifies the type of a content block within a message.
type ContentBlockType string

const (
	BlockText         ContentBlockType = "text"
	BlockToolUse      ContentBlockType = "tool_use"
	BlockToolResult   ContentBlockType = "tool_result"
	BlockThinking     ContentBlockType = "thinking"
	BlockImage        ContentBlockType = "image"
	BlockRedactedThinking ContentBlockType = "redacted_thinking"
)

// RenderableMessage is the canonical message structure used for display.
// It unifies engine messages, tool events, system notices, etc.
type RenderableMessage struct {
	UUID        string
	Type        MessageType
	Subtype     string
	Content     []ContentBlock
	Timestamp   time.Time
	IsMeta      bool
	IsStreaming bool

	// Tool use fields
	ToolUseID string
	ToolName  string
	ToolInput map[string]interface{}

	// Tool result fields
	ToolResult string
	IsError    bool
	ExitCode   int

	// Thinking
	ThinkingText string

	// File context
	FilePath string
	DiffText string
}

// ContentBlock is a single block within a message's content array.
type ContentBlock struct {
	Type     ContentBlockType
	Text     string
	Thinking string
	ToolUse  *ToolUseBlock
	ImageURL string
}

// ToolUseBlock represents a tool call within a content block.
type ToolUseBlock struct {
	ID    string
	Name  string
	Input map[string]interface{}
}

// PlainText returns the concatenated text content of all text blocks.
func (m *RenderableMessage) PlainText() string {
	var text string
	for _, b := range m.Content {
		if b.Type == BlockText {
			text += b.Text
		}
	}
	return text
}

// HasToolUse returns true if any content block is a tool_use block.
func (m *RenderableMessage) HasToolUse() bool {
	for _, b := range m.Content {
		if b.Type == BlockToolUse {
			return true
		}
	}
	return false
}

// StreamingToolUse tracks an in-progress tool use for streaming display.
type StreamingToolUse struct {
	ID       string
	Name     string
	Input    string // partial JSON input being streamed
	Started  time.Time
	Finished bool
	Output   string
	IsError  bool
}

// StreamingThinking tracks an in-progress thinking block.
type StreamingThinking struct {
	Text     string
	Started  time.Time
	Finished bool
}
