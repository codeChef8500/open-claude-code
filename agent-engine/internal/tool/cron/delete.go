package cron

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// CronDeleteInput is the input schema for CronDeleteTool.
type CronDeleteInput struct {
	ID string `json:"id"`
}

// CronDeleteTool cancels a scheduled cron job by ID.
// Aligned with claude-code-main tools/ScheduleCronTool/CronDeleteTool.ts.
type CronDeleteTool struct{ tool.BaseTool }

func NewCronDelete() *CronDeleteTool { return &CronDeleteTool{} }

func (t *CronDeleteTool) Name() string            { return "CronDelete" }
func (t *CronDeleteTool) UserFacingName() string  { return "cron_delete" }
func (t *CronDeleteTool) MaxResultSizeChars() int { return 2_000 }

func (t *CronDeleteTool) Description() string {
	return "Cancel a scheduled cron job by its ID."
}

func (t *CronDeleteTool) IsEnabled(uctx *tool.UseContext) bool {
	return isKairosOrSchedulingEnabled(uctx)
}

func (t *CronDeleteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"id": {
				"type": "string",
				"description": "The ID of the cron job to cancel."
			}
		},
		"required": ["id"]
	}`)
}

func (t *CronDeleteTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *CronDeleteTool) CheckPermissions(_ context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in CronDeleteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.ID == "" {
		return fmt.Errorf("id is required")
	}

	store := getCronStore(uctx)
	if store == nil {
		return fmt.Errorf("cron store not available")
	}

	task := store.Find(in.ID)
	if task == nil {
		return fmt.Errorf("cron job %q not found", in.ID)
	}

	// Teammates can only delete their own crons
	if agentID := getTeamAgentID(uctx); agentID != "" {
		if task.AgentID != "" && task.AgentID != agentID {
			return fmt.Errorf("cannot delete another agent's cron job")
		}
	}

	return nil
}

func (t *CronDeleteTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in CronDeleteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		store := getCronStore(uctx)
		if store == nil {
			ch <- errBlock("cron store not available")
			return
		}

		if err := store.Remove([]string{in.ID}); err != nil {
			ch <- errBlock("failed to remove: " + err.Error())
			return
		}

		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Cancelled cron job %q", in.ID),
		}
	}()
	return ch, nil
}
