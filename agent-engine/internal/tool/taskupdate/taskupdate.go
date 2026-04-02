package taskupdate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	ID          string `json:"id"`
	Status      string `json:"status,omitempty"`      // "pending"|"in_progress"|"completed"|"cancelled"
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`    // "high"|"medium"|"low"
}

type TaskUpdateTool struct{ tool.BaseTool }

func New() *TaskUpdateTool { return &TaskUpdateTool{} }

func (t *TaskUpdateTool) Name() string           { return "TaskUpdate" }
func (t *TaskUpdateTool) UserFacingName() string { return "task_update" }
func (t *TaskUpdateTool) Description() string {
	return "Update a task's status, title, description, or priority."
}
func (t *TaskUpdateTool) IsReadOnly(_ json.RawMessage) bool                  { return false }
func (t *TaskUpdateTool) IsConcurrencySafe(_ json.RawMessage) bool           { return true }
func (t *TaskUpdateTool) MaxResultSizeChars() int           { return 2048 }
func (t *TaskUpdateTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *TaskUpdateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"id":{"type":"string","description":"Task ID to update."},
			"status":{"type":"string","enum":["pending","in_progress","completed","cancelled"],"description":"New status."},
			"title":{"type":"string","description":"Updated title."},
			"description":{"type":"string","description":"Updated description."},
			"priority":{"type":"string","enum":["high","medium","low"],"description":"Updated priority."}
		},
		"required":["id"]
	}`)
}

func (t *TaskUpdateTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *TaskUpdateTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	return nil
}

func (t *TaskUpdateTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 1)
	go func() {
		defer close(ch)
		if uctx == nil || uctx.TaskRegistry == nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: `{"error":"task registry not available"}`, IsError: true}
			return
		}
		fields := map[string]interface{}{}
		if in.Status != "" {
			fields["status"] = in.Status
		}
		if in.Title != "" {
			fields["title"] = in.Title
		}
		if in.Description != "" {
			fields["description"] = in.Description
		}
		if in.Priority != "" {
			fields["priority"] = in.Priority
		}
		if err := uctx.TaskRegistry.Update(in.ID, fields); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: fmt.Sprintf(`{"error":%q}`, err.Error()), IsError: true}
			return
		}
		task, _ := uctx.TaskRegistry.Get(in.ID)
		out, _ := json.MarshalIndent(task, "", "  ")
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(out)}
	}()
	return ch, nil
}
