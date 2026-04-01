package todo

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

type TodoItem struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Status   string `json:"status"` // "pending" | "in_progress" | "completed"
	Priority string `json:"priority"` // "high" | "medium" | "low"
}

type Input struct {
	Todos []TodoItem `json:"todos"`
}

type TodoWriteTool struct{}

func New() *TodoWriteTool { return &TodoWriteTool{} }

func (t *TodoWriteTool) Name() string            { return "TodoWrite" }
func (t *TodoWriteTool) UserFacingName() string  { return "todo_write" }
func (t *TodoWriteTool) Description() string     { return "Create or update a structured todo list." }
func (t *TodoWriteTool) IsReadOnly() bool        { return false }
func (t *TodoWriteTool) IsConcurrencySafe() bool { return false }
func (t *TodoWriteTool) MaxResultSizeChars() int { return 0 }
func (t *TodoWriteTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *TodoWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"todos":{"type":"array","items":{
				"type":"object",
				"properties":{
					"id":{"type":"string"},
					"content":{"type":"string"},
					"status":{"type":"string","enum":["pending","in_progress","completed"]},
					"priority":{"type":"string","enum":["high","medium","low"]}
				},
				"required":["id","content","status","priority"]
			}}
		},
		"required":["todos"]
	}`)
}

func (t *TodoWriteTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *TodoWriteTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *TodoWriteTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		path := filepath.Join(uctx.WorkDir, ".claude", "todos.json")
		b, err := json.MarshalIndent(in.Todos, "", "  ")
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}
		if err := util.WriteTextContent(path, string(b)); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}

		summary := buildSummary(in.Todos)
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: summary}
	}()
	return ch, nil
}

func buildSummary(todos []TodoItem) string {
	var sb strings.Builder
	counts := map[string]int{"pending": 0, "in_progress": 0, "completed": 0}
	for _, t := range todos {
		counts[t.Status]++
	}
	fmt.Fprintf(&sb, "Todo list updated: %d total (%d pending, %d in progress, %d completed)",
		len(todos), counts["pending"], counts["in_progress"], counts["completed"])
	return sb.String()
}
