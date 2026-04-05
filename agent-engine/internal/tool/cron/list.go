package cron

import (
	"context"
	"encoding/json"

	"github.com/wall-ai/agent-engine/internal/daemon"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// CronListTool lists all active cron jobs.
// Aligned with claude-code-main tools/ScheduleCronTool/CronListTool.ts.
type CronListTool struct{ tool.BaseTool }

func NewCronList() *CronListTool { return &CronListTool{} }

func (t *CronListTool) Name() string           { return "CronList" }
func (t *CronListTool) UserFacingName() string  { return "cron_list" }
func (t *CronListTool) MaxResultSizeChars() int { return 10_000 }

func (t *CronListTool) Description() string {
	return "List all active cron jobs."
}

func (t *CronListTool) IsEnabled(uctx *tool.UseContext) bool {
	return isKairosOrSchedulingEnabled(uctx)
}

func (t *CronListTool) IsReadOnly(_ json.RawMessage) bool { return true }

func (t *CronListTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}

func (t *CronListTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *CronListTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *CronListTool) Call(_ context.Context, _ json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		store := getCronStore(uctx)
		if store == nil {
			ch <- errBlock("cron store not available")
			return
		}

		tasks := store.ListAll()

		// Teammate filtering: teammates only see their own crons
		agentID := getTeamAgentID(uctx)
		if agentID != "" {
			var filtered []*daemon.CronTask
			for _, task := range tasks {
				if task.AgentID == "" || task.AgentID == agentID {
					filtered = append(filtered, task)
				}
			}
			tasks = filtered
		}

		type jobEntry struct {
			ID            string `json:"id"`
			Cron          string `json:"cron"`
			HumanSchedule string `json:"humanSchedule"`
			Prompt        string `json:"prompt"`
			Recurring     bool   `json:"recurring"`
			Durable       bool   `json:"durable"`
		}

		jobs := make([]jobEntry, 0, len(tasks))
		for _, task := range tasks {
			jobs = append(jobs, jobEntry{
				ID:            task.ID,
				Cron:          task.Cron,
				HumanSchedule: daemon.CronToHuman(task.Cron),
				Prompt:        task.Prompt,
				Recurring:     task.Recurring,
				Durable:       task.Durable,
			})
		}

		result := map[string]interface{}{
			"jobs": jobs,
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(b)}
	}()
	return ch, nil
}
