package sendmessage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Message string `json:"message"`
	// Target agent ID for multi-agent message passing.
	To string `json:"to,omitempty"`
}

type SendMessageTool struct{}

func New() *SendMessageTool { return &SendMessageTool{} }

func (t *SendMessageTool) Name() string            { return "SendMessage" }
func (t *SendMessageTool) UserFacingName() string  { return "send_message" }
func (t *SendMessageTool) Description() string     { return "Send a message to the parent agent or another agent." }
func (t *SendMessageTool) IsReadOnly() bool        { return true }
func (t *SendMessageTool) IsConcurrencySafe() bool { return true }
func (t *SendMessageTool) MaxResultSizeChars() int { return 0 }
func (t *SendMessageTool) IsEnabled(uctx *tool.UseContext) bool {
	return uctx.AgentID != ""
}

func (t *SendMessageTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"message":{"type":"string","description":"Message content to send."},
			"to":{"type":"string","description":"Target agent ID. Omit to send to parent."}
		},
		"required":["message"]
	}`)
}

func (t *SendMessageTool) Prompt(_ *tool.UseContext) string { return "" }

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

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		if uctx.SendNotification != nil {
			uctx.SendNotification(in.Message)
		}
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Message sent: %s", in.Message),
		}
	}()
	return ch, nil
}
