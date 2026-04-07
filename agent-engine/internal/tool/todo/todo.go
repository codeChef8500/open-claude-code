package todo

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/state"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

type TodoItem struct {
	ID         string `json:"id"`
	Content    string `json:"content"`
	Status     string `json:"status"`               // "pending" | "in_progress" | "completed"
	Priority   string `json:"priority"`             // "high" | "medium" | "low"
	ActiveForm string `json:"activeForm,omitempty"` // present continuous form for spinner display
}

type Input struct {
	Todos []TodoItem `json:"todos"`
}

// Output is the structured output of a TodoWrite call.
type Output struct {
	OldTodos                []TodoItem `json:"oldTodos"`
	NewTodos                []TodoItem `json:"newTodos"`
	VerificationNudgeNeeded bool       `json:"verificationNudgeNeeded,omitempty"`
}

type TodoWriteTool struct{ tool.BaseTool }

func New() *TodoWriteTool { return &TodoWriteTool{} }

func (t *TodoWriteTool) Name() string                             { return "TodoWrite" }
func (t *TodoWriteTool) UserFacingName() string                   { return "" }
func (t *TodoWriteTool) Description() string                      { return "Update the todo list for the current session." }
func (t *TodoWriteTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *TodoWriteTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *TodoWriteTool) MaxResultSizeChars() int                  { return 100_000 }
func (t *TodoWriteTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *TodoWriteTool) ShouldDefer() bool                        { return true }
func (t *TodoWriteTool) SearchHint() string                       { return "manage the session task checklist" }
func (t *TodoWriteTool) Strict() bool                             { return true }
func (t *TodoWriteTool) ToAutoClassifierInput(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return fmt.Sprintf("%d items", len(in.Todos))
}

func (t *TodoWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"todos":{"type":"array","items":{
				"type":"object",
				"properties":{
					"id":{"type":"string","description":"Unique identifier for the todo item."},
					"content":{"type":"string","description":"The imperative form describing what needs to be done."},
					"status":{"type":"string","enum":["pending","in_progress","completed"],"description":"Current status of the task."},
					"priority":{"type":"string","enum":["high","medium","low"],"description":"Priority level."},
					"activeForm":{"type":"string","description":"Present continuous form shown during execution (e.g. Running tests)."}
				},
				"required":["id","content","status","priority"]
			},"description":"The updated todo list."}
		},
		"required":["todos"]
	}`)
}

func (t *TodoWriteTool) OutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"oldTodos":{"type":"array","items":{"type":"object"},"description":"The todo list before the update."},
			"newTodos":{"type":"array","items":{"type":"object"},"description":"The todo list after the update."},
			"verificationNudgeNeeded":{"type":"boolean","description":"Whether verification is recommended."}
		}
	}`)
}

func (t *TodoWriteTool) Prompt(_ *tool.UseContext) string {
	return `Use this tool to create and manage a structured task list for your current coding session. This helps you track progress, organize complex tasks, and demonstrate thoroughness to the user.
It also helps the user understand the progress of the task and overall progress of their requests.

## When to Use This Tool
Use this tool proactively in these scenarios:

1. Complex multi-step tasks - When a task requires 3 or more distinct steps or actions
2. Non-trivial and complex tasks - Tasks that require careful planning or multiple operations
3. User explicitly requests todo list - When the user directly asks you to use the todo list
4. User provides multiple tasks - When users provide a list of things to be done (numbered or comma-separated)
5. After receiving new instructions - Immediately capture user requirements as todos
6. When you start working on a task - Mark it as in_progress BEFORE beginning work. Ideally you should only have one todo as in_progress at a time
7. After completing a task - Mark it as completed and add any new follow-up tasks discovered during implementation

## When NOT to Use This Tool

Skip using this tool when:
1. There is only a single, straightforward task
2. The task is trivial and tracking it provides no organizational benefit
3. The task can be completed in less than 3 trivial steps
4. The task is purely conversational or informational

NOTE that you should not use this tool if there is only one trivial task to do. In this case you are better off just doing the task directly.

## Task States and Management

1. **Task States**: Use these states to track progress:
   - pending: Task not yet started
   - in_progress: Currently working on (limit to ONE task at a time)
   - completed: Task finished successfully

   **IMPORTANT**: Task descriptions must have two forms:
   - content: The imperative form describing what needs to be done (e.g., "Run tests", "Build the project")
   - activeForm: The present continuous form shown during execution (e.g., "Running tests", "Building the project")

2. **Task Management**:
   - Update task status in real-time as you work
   - Mark tasks complete IMMEDIATELY after finishing (don't batch completions)
   - Exactly ONE task must be in_progress at any time (not less, not more)
   - Complete current tasks before starting new ones
   - Remove tasks that are no longer relevant from the list entirely

3. **Task Completion Requirements**:
   - ONLY mark a task as completed when you have FULLY accomplished it
   - If you encounter errors, blockers, or cannot finish, keep the task as in_progress
   - When blocked, create a new task describing what needs to be resolved
   - Never mark a task as completed if:
     - Tests are failing
     - Implementation is partial
     - You encountered unresolved errors
     - You couldn't find necessary files or dependencies

4. **Task Breakdown**:
   - Create specific, actionable items
   - Break complex tasks into smaller, manageable steps
   - Use clear, descriptive task names
   - Always provide both forms:
     - content: "Fix authentication bug"
     - activeForm: "Fixing authentication bug"

When in doubt, use this tool. Being proactive with task management demonstrates attentiveness and ensures you complete all requirements successfully.`
}

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

		// Read old todos for output.
		path := filepath.Join(uctx.WorkDir, ".claude", "todos.json")
		oldTodos := readExistingTodos(path)

		// allDone: if every todo is completed, clear the list.
		allDone := len(in.Todos) > 0
		for _, td := range in.Todos {
			if td.Status != "completed" {
				allDone = false
				break
			}
		}
		newTodos := in.Todos
		if allDone {
			newTodos = []TodoItem{}
		}

		// Verification nudge: if closing 3+ items and none was a verification step.
		verificationNudgeNeeded := false
		if allDone && len(in.Todos) >= 3 {
			hasVerif := false
			for _, td := range in.Todos {
				if containsIgnoreCase(td.Content, "verif") {
					hasVerif = true
					break
				}
			}
			verificationNudgeNeeded = !hasVerif
		}

		// Persist to disk.
		b, err := json.MarshalIndent(newTodos, "", "  ")
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}
		if err := util.WriteTextContent(path, string(b)); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}

		// Dual-write to AppState for real-time TUI rendering.
		if uctx.SetAppState != nil {
			uctx.SetAppState(func(prev interface{}) interface{} {
				if appState, ok := prev.(*state.AppState); ok {
					items := make([]state.TodoItemState, len(newTodos))
					for i, td := range newTodos {
						items[i] = state.TodoItemState{
							ID:         td.ID,
							Content:    td.Content,
							Status:     td.Status,
							Priority:   td.Priority,
							ActiveForm: td.ActiveForm,
						}
					}
					appState.TodoItems = items
				}
				return prev
			})
		}

		// Build structured output.
		out := Output{
			OldTodos:                oldTodos,
			NewTodos:                in.Todos,
			VerificationNudgeNeeded: verificationNudgeNeeded,
		}

		// Tool result text for the model.
		resultText := "Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current tasks if applicable"
		if verificationNudgeNeeded {
			resultText += "\n\nNOTE: You just closed out 3+ tasks and none of them was a verification step. Before writing your final summary, consider spawning a verification agent to validate your work."
		}

		// Also marshal the structured output for downstream consumers.
		ob, _ := json.Marshal(out)
		_ = ob // structured data available if needed

		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: resultText}
	}()
	return ch, nil
}

// MapToolResultToBlockParam formats the todo result for the model.
func (t *TodoWriteTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text, ok := content.(string)
	if !ok {
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: ""}
	}
	return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: text}
}

// readExistingTodos reads the current todos from disk, returns nil on any error.
func readExistingTodos(path string) []TodoItem {
	data, err := util.ReadTextFile(path)
	if err != nil {
		return nil
	}
	var todos []TodoItem
	if json.Unmarshal([]byte(data), &todos) != nil {
		return nil
	}
	return todos
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
