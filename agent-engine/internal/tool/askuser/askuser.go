package askuser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// Option is a predefined answer choice with label and description.
type Option struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type Input struct {
	Question      string   `json:"question"`
	Options       []Option `json:"options,omitempty"`
	AllowMultiple bool     `json:"allowMultiple,omitempty"`
}

type AskUserTool struct{ tool.BaseTool }

func New() *AskUserTool { return &AskUserTool{} }

func (t *AskUserTool) Name() string           { return "AskUser" }
func (t *AskUserTool) UserFacingName() string { return "ask_user" }
func (t *AskUserTool) Description() string {
	return "Ask the user a question and wait for their response."
}
func (t *AskUserTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *AskUserTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *AskUserTool) MaxResultSizeChars() int                  { return 0 }
func (t *AskUserTool) IsEnabled(_ *tool.UseContext) bool        { return true }

func (t *AskUserTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"question":{"type":"string","description":"The question to ask the user."},
			"options":{"type":"array","items":{"type":"object","properties":{"label":{"type":"string","description":"Short label for the option."},"description":{"type":"string","description":"Longer description explaining the option."}},"required":["label","description"]},"description":"Up to 4 options for the user to choose from."},
			"allowMultiple":{"type":"boolean","description":"Whether the user can select multiple options."}
		},
		"required":["question","options","allowMultiple"]
	}`)
}

func (t *AskUserTool) Prompt(_ *tool.UseContext) string {
	return `Ask the user a question with predefined options. Use this when you need the user to make a choice between specific options.
You can provide up to 4 options, each with a label and description.
NEVER include "other" as an option - the user can always automatically provide a custom response.
Set allowMultiple to true if the user should be able to select more than one option.

Usage:
- Ask clear, specific questions that help you understand the user's intent
- Provide predefined options when the answer space is limited
- Avoid asking unnecessary questions — use your best judgment first
- This tool pauses execution until the user responds
- The user may provide a custom response instead of selecting an option`
}

func (t *AskUserTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Question == "" {
		return fmt.Errorf("question must not be empty")
	}
	return nil
}

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

		// Build a summary of the question for display.
		summary := in.Question
		if len(in.Options) > 0 {
			summary += " ["
			for i, opt := range in.Options {
				if i > 0 {
					summary += ", "
				}
				summary += opt.Label
			}
			summary += "]"
		}

		// Use RequestPrompt if available (interactive prompt elicitation).
		if uctx.RequestPrompt != nil {
			promptFn := uctx.RequestPrompt(t.Name(), summary)
			if promptFn != nil {
				resp, err := promptFn(map[string]interface{}{
					"question":      in.Question,
					"options":       in.Options,
					"allowMultiple": in.AllowMultiple,
				})
				if err != nil {
					ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
					return
				}
				// Marshal response to string.
				if s, ok := resp.(string); ok {
					ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: s}
				} else {
					data, _ := json.Marshal(resp)
					ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(data)}
				}
				return
			}
		}

		// Fallback: if AskPermission callback, use it.
		if uctx.AskPermission != nil {
			approved, err := uctx.AskPermission(ctx, t.Name(), summary)
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
