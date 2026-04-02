package planmode

import (
	"context"
	"encoding/json"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// ─── EnterPlanMode ────────────────────────────────────────────────────────────

// EnterPlanModeTool signals that the agent should enter plan mode,
// pausing tool execution and presenting a plan for user approval.
type EnterPlanModeTool struct{ tool.BaseTool }

func NewEnterPlanMode() *EnterPlanModeTool { return &EnterPlanModeTool{} }

func (t *EnterPlanModeTool) Name() string           { return "enter_plan_mode" }
func (t *EnterPlanModeTool) UserFacingName() string { return "EnterPlanMode" }
func (t *EnterPlanModeTool) Description() string {
	return "Enter plan mode: pause execution and present a structured plan for user review and approval before proceeding."
}
func (t *EnterPlanModeTool) IsReadOnly() bool                  { return true }
func (t *EnterPlanModeTool) IsConcurrencySafe() bool           { return true }
func (t *EnterPlanModeTool) MaxResultSizeChars() int           { return 1000 }
func (t *EnterPlanModeTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *EnterPlanModeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"plan": {
				"type": "string",
				"description": "The plan to present to the user for approval."
			}
		},
		"required": ["plan"]
	}`)
}

func (t *EnterPlanModeTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *EnterPlanModeTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *EnterPlanModeTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		var args struct {
			Plan string `json:"plan"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "invalid input: " + err.Error(), IsError: true}
			return
		}
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: "[PLAN MODE]\n\n" + args.Plan + "\n\nPlease review and approve before I proceed.",
		}
	}()
	return ch, nil
}

// ─── ExitPlanMode ─────────────────────────────────────────────────────────────

// ExitPlanModeTool signals that plan mode has ended and normal execution resumes.
type ExitPlanModeTool struct{ tool.BaseTool }

func NewExitPlanMode() *ExitPlanModeTool { return &ExitPlanModeTool{} }

func (t *ExitPlanModeTool) Name() string           { return "exit_plan_mode" }
func (t *ExitPlanModeTool) UserFacingName() string { return "ExitPlanMode" }
func (t *ExitPlanModeTool) Description() string {
	return "Exit plan mode and resume normal tool execution."
}
func (t *ExitPlanModeTool) IsReadOnly() bool                  { return true }
func (t *ExitPlanModeTool) IsConcurrencySafe() bool           { return true }
func (t *ExitPlanModeTool) MaxResultSizeChars() int           { return 200 }
func (t *ExitPlanModeTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *ExitPlanModeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

func (t *ExitPlanModeTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *ExitPlanModeTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *ExitPlanModeTool) Call(_ context.Context, _ json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 1)
	go func() {
		defer close(ch)
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Plan mode exited. Resuming normal execution."}
	}()
	return ch, nil
}
