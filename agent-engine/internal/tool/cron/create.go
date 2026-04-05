package cron

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/daemon"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

const maxCronJobs = 50

// CronCreateInput is the input schema for CronCreateTool.
type CronCreateInput struct {
	Cron      string `json:"cron"`
	Prompt    string `json:"prompt"`
	Recurring bool   `json:"recurring"`
	Durable   bool   `json:"durable"`
}

// CronCreateTool schedules one-shot or recurring prompts using cron expressions.
// Aligned with claude-code-main tools/ScheduleCronTool/CronCreateTool.ts.
type CronCreateTool struct{ tool.BaseTool }

func NewCronCreate() *CronCreateTool { return &CronCreateTool{} }

func (t *CronCreateTool) Name() string            { return "CronCreate" }
func (t *CronCreateTool) UserFacingName() string  { return "cron_create" }
func (t *CronCreateTool) MaxResultSizeChars() int { return 5_000 }

func (t *CronCreateTool) Description() string {
	return "Schedule a one-shot or recurring prompt using a cron expression. " +
		"Use durable=true for tasks that survive across sessions."
}

func (t *CronCreateTool) IsEnabled(uctx *tool.UseContext) bool {
	return isKairosOrSchedulingEnabled(uctx)
}

func (t *CronCreateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"cron": {
				"type": "string",
				"description": "5-field cron expression (minute hour day-of-month month day-of-week). Avoid :00/:30 to reduce clustering."
			},
			"prompt": {
				"type": "string",
				"description": "The prompt to execute when the cron fires."
			},
			"recurring": {
				"type": "boolean",
				"description": "If true, the task repeats on schedule. If false, it fires once and is removed."
			},
			"durable": {
				"type": "boolean",
				"description": "If true, the task persists across sessions (written to disk). If false, session-only."
			}
		},
		"required": ["cron", "prompt"]
	}`)
}

func (t *CronCreateTool) Prompt(_ *tool.UseContext) string {
	return `Schedule prompts using 5-field cron expressions (minute hour day-of-month month day-of-week).
Avoid scheduling on exact :00 or :30 marks — prefer odd minutes like :07, :23, :41 to reduce clustering.
Use durable=true for tasks that should survive across sessions.
Recurring tasks auto-expire after 14 days unless marked permanent by the system.`
}

func (t *CronCreateTool) CheckPermissions(_ context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in CronCreateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Cron == "" {
		return fmt.Errorf("cron expression is required")
	}
	if in.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	// Validate cron expression
	if _, err := daemon.ParseCronExpression(in.Cron); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	// Teammate cannot create durable tasks
	if in.Durable && isTeammate(uctx) {
		return fmt.Errorf("teammates cannot create durable cron tasks")
	}
	return nil
}

func (t *CronCreateTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in CronCreateInput
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

		// Check job limit
		if store.Count() >= maxCronJobs {
			ch <- errBlock(fmt.Sprintf("maximum %d cron jobs reached", maxCronJobs))
			return
		}

		agentID := getTeamAgentID(uctx)

		id, err := store.Add(in.Cron, in.Prompt, in.Recurring, in.Durable, agentID)
		if err != nil {
			ch <- errBlock("failed to add task: " + err.Error())
			return
		}

		human := daemon.CronToHuman(in.Cron)
		result := map[string]interface{}{
			"id":            id,
			"humanSchedule": human,
			"recurring":     in.Recurring,
			"durable":       in.Durable,
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(b)}
	}()
	return ch, nil
}
