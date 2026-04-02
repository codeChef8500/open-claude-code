package taskget

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	ID string `json:"id"`
}

type TaskGetTool struct{ tool.BaseTool }

func New() *TaskGetTool { return &TaskGetTool{} }

func (t *TaskGetTool) Name() string           { return "TaskGet" }
func (t *TaskGetTool) UserFacingName() string { return "task_get" }
func (t *TaskGetTool) Description() string {
	return "Retrieve a task by ID and return its current state."
}
func (t *TaskGetTool) IsReadOnly() bool                  { return true }
func (t *TaskGetTool) IsConcurrencySafe() bool           { return true }
func (t *TaskGetTool) MaxResultSizeChars() int           { return 4096 }
func (t *TaskGetTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *TaskGetTool) IsSearchOrRead() bool              { return true }

func (t *TaskGetTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"id":{"type":"string","description":"Task ID to retrieve."}
		},
		"required":["id"]
	}`)
}

func (t *TaskGetTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *TaskGetTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.ID == "" {
		return fmt.Errorf("id must not be empty")
	}
	return nil
}

func (t *TaskGetTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
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
		task, ok := uctx.TaskRegistry.Get(in.ID)
		if !ok {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf(`{"error":"task %q not found"}`, in.ID),
				IsError: true,
			}
			return
		}
		out, _ := json.MarshalIndent(task, "", "  ")
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(out)}
	}()
	return ch, nil
}
