package taskoutput

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// TaskOutputProvider is the interface for retrieving task outputs.
// The actual implementation is injected by the application layer.
type TaskOutputProvider interface {
	// GetTaskOutput retrieves the output for a given task ID.
	// Returns the output string, the task status, and any error.
	GetTaskOutput(ctx context.Context, taskID string) (output string, status string, err error)
	// IsTaskComplete returns true if the task has finished.
	IsTaskComplete(ctx context.Context, taskID string) (bool, error)
}

// TaskOutputTool retrieves output from background tasks.
type TaskOutputTool struct {
	tool.BaseTool
	provider TaskOutputProvider
}

// New creates a TaskOutputTool with the given provider.
func New(provider TaskOutputProvider) *TaskOutputTool {
	return &TaskOutputTool{provider: provider}
}

func (t *TaskOutputTool) Name() string           { return "task_output" }
func (t *TaskOutputTool) UserFacingName() string  { return "TaskOutput" }
func (t *TaskOutputTool) Description() string {
	return "Get the output from a running or completed background task. Can optionally block until the task completes."
}
func (t *TaskOutputTool) MaxResultSizeChars() int                  { return 100_000 }
func (t *TaskOutputTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *TaskOutputTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *TaskOutputTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *TaskOutputTool) ShouldDefer() bool                        { return true }
func (t *TaskOutputTool) SearchHint() string                       { return "get output from a background task" }

func (t *TaskOutputTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task_id": {
				"type": "string",
				"description": "The task ID to get output from."
			},
			"block": {
				"type": "boolean",
				"description": "Whether to wait for completion (default: true).",
				"default": true
			},
			"timeout": {
				"type": "number",
				"description": "Max wait time in milliseconds (default: 30000, max: 600000).",
				"default": 30000
			}
		},
		"required": ["task_id"]
	}`)
}

func (t *TaskOutputTool) Prompt(_ *tool.UseContext) string {
	return "Use this tool to check on and retrieve output from background tasks (bash commands, agent tasks, etc.)."
}

func (t *TaskOutputTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *TaskOutputTool) Call(ctx context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)

	go func() {
		defer close(ch)

		var args struct {
			TaskID  string   `json:"task_id"`
			Block   *bool    `json:"block"`
			Timeout *float64 `json:"timeout"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "invalid input: " + err.Error(), IsError: true}
			return
		}
		if args.TaskID == "" {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "task_id is required", IsError: true}
			return
		}

		block := true
		if args.Block != nil {
			block = *args.Block
		}

		timeoutMs := 30000.0
		if args.Timeout != nil {
			timeoutMs = *args.Timeout
		}
		if timeoutMs < 0 {
			timeoutMs = 0
		}
		if timeoutMs > 600000 {
			timeoutMs = 600000
		}

		if block && timeoutMs > 0 {
			// Poll until complete or timeout.
			deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
			pollInterval := 500 * time.Millisecond

			for time.Now().Before(deadline) {
				complete, err := t.provider.IsTaskComplete(ctx, args.TaskID)
				if err != nil {
					ch <- &engine.ContentBlock{
						Type:    engine.ContentTypeText,
						Text:    fmt.Sprintf("Error checking task %s: %v", args.TaskID, err),
						IsError: true,
					}
					return
				}
				if complete {
					break
				}

				select {
				case <-ctx.Done():
					ch <- &engine.ContentBlock{
						Type:    engine.ContentTypeText,
						Text:    "Cancelled while waiting for task.",
						IsError: true,
					}
					return
				case <-time.After(pollInterval):
				}
			}
		}

		output, status, err := t.provider.GetTaskOutput(ctx, args.TaskID)
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Error getting output for task %s: %v", args.TaskID, err),
				IsError: true,
			}
			return
		}

		// Truncate if needed.
		if len(output) > 100_000 {
			output = output[:100_000] + "\n... [truncated]"
		}

		result := map[string]interface{}{
			"task_id": args.TaskID,
			"status":  status,
			"output":  output,
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: string(data),
		}
	}()

	return ch, nil
}
