package askuser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}

type AskUserTool struct{ tool.BaseTool }

func New() *AskUserTool { return &AskUserTool{} }

func (t *AskUserTool) Name() string           { return "AskUser" }
func (t *AskUserTool) UserFacingName() string { return "ask_user" }
func (t *AskUserTool) Description() string {
	return "Ask the user a question and wait for their response."
}
func (t *AskUserTool) IsReadOnly() bool                  { return true }
func (t *AskUserTool) IsConcurrencySafe() bool           { return false }
func (t *AskUserTool) MaxResultSizeChars() int           { return 0 }
func (t *AskUserTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *AskUserTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"question":{"type":"string","description":"The question to ask the user."},
			"options":{"type":"array","items":{"type":"string"},"description":"Optional predefined answer choices."}
		},
		"required":["question"]
	}`)
}

func (t *AskUserTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *AskUserTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Question == "" {
		return fmt.Errorf("question must not be empty")
	}
	return nil
}

func (t *AskUserTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		// If the caller provided an AskPermission callback, use it.
		if uctx.AskPermission != nil {
			approved, err := uctx.AskPermission(ctx, t.Name(), in.Question)
			if err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
				return
			}
			answer := "no"
			if approved {
				answer = "yes"
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: answer}
			return
		}

		// No interactive channel — return a placeholder response.
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: "[awaiting user response to: " + in.Question + "]",
		}
	}()
	return ch, nil
}
