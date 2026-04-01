package taskstop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Reason string `json:"reason,omitempty"`
}

type TaskStopTool struct{}

func New() *TaskStopTool { return &TaskStopTool{} }

func (t *TaskStopTool) Name() string            { return "TaskStop" }
func (t *TaskStopTool) UserFacingName() string  { return "task_stop" }
func (t *TaskStopTool) Description() string     { return "Signal that the current task is complete and stop the agent loop." }
func (t *TaskStopTool) IsReadOnly() bool        { return true }
func (t *TaskStopTool) IsConcurrencySafe() bool { return false }
func (t *TaskStopTool) MaxResultSizeChars() int { return 0 }
func (t *TaskStopTool) IsEnabled(uctx *tool.UseContext) bool {
	return uctx.AgentID != ""
}

func (t *TaskStopTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"reason":{"type":"string","description":"Optional reason for stopping the task."}
		}
	}`)
}

func (t *TaskStopTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *TaskStopTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *TaskStopTool) Call(_ context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	_ = json.Unmarshal(input, &in)

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		msg := "Task stopped."
		if in.Reason != "" {
			msg = fmt.Sprintf("Task stopped: %s", in.Reason)
		}
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: msg}
	}()
	return ch, nil
}
