package tasklist

import (
	"context"
	"encoding/json"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Status   string `json:"status,omitempty"`   // filter by status
	Priority string `json:"priority,omitempty"` // filter by priority
}

type TaskListTool struct{ tool.BaseTool }

func New() *TaskListTool { return &TaskListTool{} }

func (t *TaskListTool) Name() string           { return "TaskList" }
func (t *TaskListTool) UserFacingName() string { return "task_list" }
func (t *TaskListTool) Description() string {
	return "List all tracked tasks, optionally filtered by status or priority."
}
func (t *TaskListTool) IsReadOnly() bool                  { return true }
func (t *TaskListTool) IsConcurrencySafe() bool           { return true }
func (t *TaskListTool) MaxResultSizeChars() int           { return 16_000 }
func (t *TaskListTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *TaskListTool) IsSearchOrRead() bool              { return true }

func (t *TaskListTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"status":{"type":"string","enum":["pending","in_progress","completed","cancelled"],"description":"Filter by status."},
			"priority":{"type":"string","enum":["high","medium","low"],"description":"Filter by priority."}
		}
	}`)
}

func (t *TaskListTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *TaskListTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *TaskListTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	_ = json.Unmarshal(input, &in)

	ch := make(chan *engine.ContentBlock, 1)
	go func() {
		defer close(ch)
		if uctx == nil || uctx.TaskRegistry == nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: `[]`}
			return
		}
		tasks := uctx.TaskRegistry.List()
		// Apply filters.
		var filtered []map[string]interface{}
		for _, task := range tasks {
			if in.Status != "" {
				if s, _ := task["status"].(string); s != in.Status {
					continue
				}
			}
			if in.Priority != "" {
				if p, _ := task["priority"].(string); p != in.Priority {
					continue
				}
			}
			filtered = append(filtered, task)
		}
		if filtered == nil {
			filtered = []map[string]interface{}{}
		}
		out, _ := json.MarshalIndent(filtered, "", "  ")
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(out)}
	}()
	return ch, nil
}
