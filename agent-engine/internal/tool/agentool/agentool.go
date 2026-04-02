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

func (t *AgentTool) Name() string                      { return "Task" }
func (t *AgentTool) UserFacingName() string            { return "task" }
func (t *AgentTool) Description() string               { return "Spawn a sub-agent to complete a task autonomously." }
func (t *AgentTool) IsReadOnly() bool                  { return false }
func (t *AgentTool) IsConcurrencySafe() bool           { return true }
func (t *AgentTool) MaxResultSizeChars() int           { return 50_000 }
func (t *AgentTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *AgentTool) IsTransparentWrapper() bool        { return true }

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
	return `## Task Tool
Spawn a sub-agent to complete a focused task. The sub-agent runs autonomously and returns its result.
Use this for parallelisable or delegatable subtasks.`
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
