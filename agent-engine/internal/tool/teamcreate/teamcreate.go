package teamcreate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// TeamCreateTool creates a named team of sub-agents.
type TeamCreateTool struct{ tool.BaseTool }

func New() *TeamCreateTool { return &TeamCreateTool{} }

func (t *TeamCreateTool) Name() string           { return "team_create" }
func (t *TeamCreateTool) UserFacingName() string { return "TeamCreate" }
func (t *TeamCreateTool) Description() string {
	return "Create a named team of parallel sub-agents. Each agent receives an independent task and works concurrently."
}
func (t *TeamCreateTool) IsReadOnly(_ json.RawMessage) bool                  { return false }
func (t *TeamCreateTool) IsConcurrencySafe(_ json.RawMessage) bool           { return false }
func (t *TeamCreateTool) MaxResultSizeChars() int           { return 4000 }
func (t *TeamCreateTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *TeamCreateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"team_name": {
				"type": "string",
				"description": "Unique identifier for this team."
			},
			"agents": {
				"type": "array",
				"description": "List of agent definitions.",
				"items": {
					"type": "object",
					"properties": {
						"agent_id": {"type": "string"},
						"task":     {"type": "string"},
						"work_dir": {"type": "string"}
					},
					"required": ["agent_id", "task"]
				}
			}
		},
		"required": ["team_name", "agents"]
	}`)
}

func (t *TeamCreateTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *TeamCreateTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *TeamCreateTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		var args struct {
			TeamName string `json:"team_name"`
			Agents   []struct {
				AgentID string `json:"agent_id"`
				Task    string `json:"task"`
				WorkDir string `json:"work_dir"`
			} `json:"agents"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "invalid input: " + err.Error(), IsError: true}
			return
		}

		workDir := ""
		if uctx != nil {
			workDir = uctx.WorkDir
		}

		result := fmt.Sprintf("Team %q created with %d agent(s):\n", args.TeamName, len(args.Agents))
		for _, a := range args.Agents {
			dir := a.WorkDir
			if dir == "" {
				dir = workDir
			}
			result += fmt.Sprintf("  - %s: %s (dir: %s)\n", a.AgentID, a.Task, dir)
		}
		result += "\nAgents are queued for parallel execution."

		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: result}
	}()
	return ch, nil
}
