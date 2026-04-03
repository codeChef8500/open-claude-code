package agentool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Task         string   `json:"task"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	MaxTurns     int      `json:"max_turns,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
}

// SubAgentRunner is the callback the parent engine provides to launch a child agent.
type SubAgentRunner func(ctx context.Context, agentID, task string, input Input, uctx *tool.UseContext) (string, error)

// AgentTool spawns a sub-agent to complete a task.
type AgentTool struct {
	tool.BaseTool
	runSubAgent SubAgentRunner
}

func New(runner SubAgentRunner) *AgentTool {
	return &AgentTool{runSubAgent: runner}
}

func (t *AgentTool) Name() string                             { return "Task" }
func (t *AgentTool) UserFacingName() string                   { return "task" }
func (t *AgentTool) Description() string                      { return "Spawn a sub-agent to complete a task autonomously." }
func (t *AgentTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *AgentTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *AgentTool) MaxResultSizeChars() int                  { return 50_000 }
func (t *AgentTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *AgentTool) IsTransparentWrapper() bool               { return true }

func (t *AgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"task":{"type":"string","description":"Description of the task for the sub-agent."},
			"allowed_tools":{"type":"array","items":{"type":"string"},"description":"Optional list of tool names the sub-agent may use."},
			"max_turns":{"type":"integer","description":"Maximum turns for the sub-agent (default 50)."},
			"system_prompt":{"type":"string","description":"Optional custom system prompt for the sub-agent."}
		},
		"required":["task"]
	}`)
}

func (t *AgentTool) Prompt(_ *tool.UseContext) string {
	return `Launch a new agent to handle complex, multi-step tasks autonomously.

The Task tool launches specialized agents (subprocesses) that autonomously handle complex tasks.

When NOT to use the Task tool:
- If you want to read a specific file path, use the Read tool or Glob tool instead, to find the match more quickly
- If you are searching for a specific class definition like "class Foo", use Glob/Grep instead, to find the match more quickly
- If you are searching for code within a specific file or set of 2-3 files, use the Read tool instead

Usage notes:
- Always include a short description summarizing what the agent will do
- Launch multiple agents concurrently whenever possible, to maximize performance; to do that, use a single message with multiple tool uses
- When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary.
- Each Agent invocation starts fresh — provide a complete task description.
- The agent's outputs should generally be trusted
- Clearly tell the agent whether you expect it to write code or just to do research (search, file reads, web fetches, etc.)

Writing the prompt:
- Brief the agent like a smart colleague who just walked into the room — it hasn't seen this conversation, doesn't know what you've tried, doesn't understand why this task matters.
- Explain what you're trying to accomplish and why.
- Describe what you've already learned or ruled out.
- Give enough context about the surrounding problem that the agent can make judgment calls rather than just following a narrow instruction.`
}

func (t *AgentTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Task == "" {
		return fmt.Errorf("task must not be empty")
	}
	if in.MaxTurns < 0 {
		return fmt.Errorf("max_turns must be non-negative")
	}
	if in.MaxTurns > 200 {
		return fmt.Errorf("max_turns exceeds maximum of 200")
	}
	return nil
}

func (t *AgentTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Task == "" {
		return fmt.Errorf("task must not be empty")
	}
	return nil
}

func (t *AgentTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 4)
	go func() {
		defer close(ch)

		agentID := uuid.New().String()

		if t.runSubAgent == nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    "Sub-agent runner not configured.",
				IsError: true,
			}
			return
		}

		result, err := t.runSubAgent(ctx, agentID, in.Task, in, uctx)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}

		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: result}
	}()
	return ch, nil
}
