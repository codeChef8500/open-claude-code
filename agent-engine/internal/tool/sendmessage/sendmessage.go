package sendmessage

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// Input is the JSON input schema for the SendMessage tool.
// Extended to support mailbox, broadcast, and structured messages.
type Input struct {
	Message string `json:"message"`
	// Target agent ID for multi-agent message passing.
	To string `json:"to,omitempty"`
	// MessageType for structured messages: "text", "shutdown_request",
	// "shutdown_response", "plan_approval_response".
	MessageType string `json:"message_type,omitempty"`
	// Priority: "normal", "high", "low".
	Priority string `json:"priority,omitempty"`
}

// MailboxSender abstracts mailbox delivery so we don't import agent directly.
type MailboxSender interface {
	// Send delivers a message from one agent to another via mailbox.
	Send(from, to, text string, priority string, replyTo string) (string, error)
	// Broadcast sends a message to all agents in a team.
	Broadcast(from, teamName, text string) error
	// TeamMembers returns agent IDs in a team (for broadcast).
	TeamMembers(teamName string) []string
}

// SendMessageTool sends messages between agents.
type SendMessageTool struct {
	tool.BaseTool
	sender MailboxSender
}

// New creates a SendMessageTool without mailbox integration (legacy).
func New() *SendMessageTool { return &SendMessageTool{} }

// NewWithMailbox creates a SendMessageTool with full mailbox integration.
func NewWithMailbox(sender MailboxSender) *SendMessageTool {
	return &SendMessageTool{sender: sender}
}

func (t *SendMessageTool) Name() string           { return "SendMessage" }
func (t *SendMessageTool) UserFacingName() string { return "send_message" }
func (t *SendMessageTool) Description() string {
	return "Send a message to the parent agent or another agent."
}
func (t *SendMessageTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *SendMessageTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *SendMessageTool) MaxResultSizeChars() int                  { return 0 }
func (t *SendMessageTool) IsEnabled(uctx *tool.UseContext) bool {
	return uctx.AgentID != ""
}

func (t *SendMessageTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"message":{"type":"string","description":"Message content to send."},
			"to":{"type":"string","description":"Target agent ID, 'parent' for parent agent, or '*' to broadcast to team. Omit to send to parent."},
			"message_type":{"type":"string","enum":["text","shutdown_request","shutdown_response","plan_approval_response"],"description":"Message type. Default: text."},
			"priority":{"type":"string","enum":["normal","high","low"],"description":"Message priority. Default: normal."}
		},
		"required":["message"]
	}`)
}

func (t *SendMessageTool) Prompt(_ *tool.UseContext) string {
	return `Send a message to the parent agent, another agent, or broadcast to all agents.

Usage:
- Use this tool for inter-agent communication in multi-agent setups
- Specify the "to" field with the target agent name or ID; omit to send to parent
- Use "*" as the "to" value to broadcast to all agents
- Messages can be plain text or structured (shutdown_request, shutdown_response, plan_approval_response)
- Include a short summary for UI preview when sending plain text messages`
}

func (t *SendMessageTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Message == "" {
		return fmt.Errorf("message must not be empty")
	}
	return nil
}

func (t *SendMessageTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Message == "" {
		return fmt.Errorf("message must not be empty")
	}
	return nil
}

func (t *SendMessageTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Default message type.
	if in.MessageType == "" {
		in.MessageType = "text"
	}
	if in.Priority == "" {
		in.Priority = "normal"
	}

	// Format structured messages.
	msgContent := t.formatMessage(in)

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		fromAgent := uctx.AgentID

		// ── Route the message ──────────────────────────────────────────
		switch {
		case in.To == "*":
			// Broadcast to team.
			resultText := t.handleBroadcast(fromAgent, uctx.AgentType, msgContent)
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: resultText}

		case in.To == "" || in.To == "parent":
			// Send to parent via notification callback.
			if uctx.SendNotification != nil {
				uctx.SendNotification(msgContent)
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Message sent to parent."}

		default:
			// Send to specific agent via mailbox.
			resultText := t.handleDirectSend(fromAgent, in.To, msgContent, in.Priority)
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: resultText}
		}
	}()
	return ch, nil
}

// handleBroadcast sends a message to all team members.
func (t *SendMessageTool) handleBroadcast(from, teamName, message string) string {
	if t.sender != nil {
		err := t.sender.Broadcast(from, teamName, message)
		if err != nil {
			slog.Warn("sendmessage: broadcast failed", slog.Any("err", err))
			return fmt.Sprintf("Broadcast failed: %v", err)
		}
		return "Message broadcast to team."
	}
	return "Broadcast not available (no mailbox configured)."
}

// handleDirectSend sends a message to a specific agent via mailbox.
func (t *SendMessageTool) handleDirectSend(from, to, message, priority string) string {
	if t.sender != nil {
		msgID, err := t.sender.Send(from, to, message, priority, "")
		if err != nil {
			slog.Warn("sendmessage: send failed",
				slog.String("to", to),
				slog.Any("err", err))
			return fmt.Sprintf("Send failed: %v", err)
		}
		return fmt.Sprintf("Message sent to %s (id: %s).", to, msgID)
	}
	return fmt.Sprintf("Message to %s queued (no mailbox configured).", to)
}

// formatMessage formats the message content based on type.
func (t *SendMessageTool) formatMessage(in Input) string {
	switch in.MessageType {
	case "shutdown_request":
		return "__shutdown__"
	case "shutdown_response":
		return fmt.Sprintf("__shutdown_ack__:%s", in.Message)
	case "plan_approval_response":
		return fmt.Sprintf("__plan_approval__:%s", in.Message)
	default:
		return in.Message
	}
}
