package taskcreate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"` // "high"|"medium"|"low"
}

type TaskCreateTool struct{ tool.BaseTool }

func New() *TaskCreateTool { return &TaskCreateTool{} }

func (t *TaskCreateTool) Name() string           { return "TaskCreate" }
func (t *TaskCreateTool) UserFacingName() string { return "task_create" }
func (t *TaskCreateTool) Description() string {
	return "Create a new tracked task with a title and optional description."
}
func (t *TaskCreateTool) IsReadOnly(_ json.RawMessage) bool                     { return false }
func (t *TaskCreateTool) IsConcurrencySafe(_ json.RawMessage) bool              { return true }
func (t *TaskCreateTool) MaxResultSizeChars() int              { return 2048 }
func (t *TaskCreateTool) IsEnabled(_ *tool.UseContext) bool    { return true }

func (t *TaskCreateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"title":{"type":"string","description":"Short task title."},
			"description":{"type":"string","description":"Detailed task description."},
			"priority":{"type":"string","enum":["high","medium","low"],"description":"Task priority (default: medium)."}
		},
		"required":["title"]
	}`)
}

func (t *TaskCreateTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *TaskCreateTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Title == "" {
		return fmt.Errorf("title must not be empty")
	}
	return nil
}

func (t *TaskCreateTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}

	id := uuid.New().String()
	task := map[string]interface{}{
		"id":          id,
		"title":       in.Title,
		"description": in.Description,
		"priority":    in.Priority,
		"status":      "pending",
		"created_at":  time.Now().UTC().Format(time.RFC3339),
	}

	if uctx != nil && uctx.TaskRegistry != nil {
		uctx.TaskRegistry.Create(id, in.Title, in.Description, in.Priority)
	}

	out, _ := json.MarshalIndent(task, "", "  ")
	ch := make(chan *engine.ContentBlock, 1)
	go func() {
		defer close(ch)
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(out)}
	}()
	return ch, nil
}
