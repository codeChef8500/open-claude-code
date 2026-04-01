package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	robfigcron "github.com/robfig/cron/v3"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

// ScheduledTask is a registered cron job entry.
type ScheduledTask struct {
	ID          string    `json:"id"`
	Schedule    string    `json:"schedule"`
	Command     string    `json:"command"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Scheduler wraps robfig/cron with task persistence.
type Scheduler struct {
	mu    sync.RWMutex
	c     *robfigcron.Cron
	tasks map[string]*ScheduledTask
}

var defaultScheduler *Scheduler
var once sync.Once

func getScheduler() *Scheduler {
	once.Do(func() {
		defaultScheduler = &Scheduler{
			c:     robfigcron.New(robfigcron.WithSeconds()),
			tasks: make(map[string]*ScheduledTask),
		}
		defaultScheduler.c.Start()
	})
	return defaultScheduler
}

// ─── Tool ─────────────────────────────────────────────────────────────────────

type ScheduleInput struct {
	Action      string `json:"action"`   // "add" | "remove" | "list"
	ID          string `json:"id,omitempty"`
	Schedule    string `json:"schedule,omitempty"`
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
}

type ScheduleCronTool struct{}

func New() *ScheduleCronTool { return &ScheduleCronTool{} }

func (t *ScheduleCronTool) Name() string            { return "ScheduleCron" }
func (t *ScheduleCronTool) UserFacingName() string  { return "schedule_cron" }
func (t *ScheduleCronTool) Description() string     { return "Schedule, remove, or list cron jobs." }
func (t *ScheduleCronTool) IsReadOnly() bool        { return false }
func (t *ScheduleCronTool) IsConcurrencySafe() bool { return false }
func (t *ScheduleCronTool) MaxResultSizeChars() int { return 10_000 }
func (t *ScheduleCronTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *ScheduleCronTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"action":{"type":"string","enum":["add","remove","list"]},
			"id":{"type":"string","description":"Job ID (required for remove)"},
			"schedule":{"type":"string","description":"Cron expression (e.g. '0 * * * *')"},
			"command":{"type":"string","description":"Shell command to run"},
			"description":{"type":"string","description":"Human-readable description"}
		},
		"required":["action"]
	}`)
}

func (t *ScheduleCronTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *ScheduleCronTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in ScheduleInput
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Action == "" {
		return fmt.Errorf("action must not be empty")
	}
	return nil
}

func (t *ScheduleCronTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in ScheduleInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		sched := getScheduler()

		switch in.Action {
		case "add":
			if in.Schedule == "" || in.Command == "" {
				ch <- errBlock("schedule and command required for add")
				return
			}
			id := in.ID
			if id == "" {
				id = fmt.Sprintf("job-%d", time.Now().UnixNano())
			}
			cmd := in.Command
			wdir := uctx.WorkDir
			notify := uctx.SendNotification
			_, err := sched.c.AddFunc(in.Schedule, func() {
				runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				res, execErr := util.Exec(runCtx, cmd, &util.ExecOptions{CWD: wdir})
				if notify != nil {
					if execErr != nil {
						notify(fmt.Sprintf("[cron %s] error: %s", id, execErr.Error()))
					} else {
						out := res.Stdout
						if out == "" {
							out = "(no output)"
						}
						notify(fmt.Sprintf("[cron %s] %s", id, out))
					}
				}
			})
			if err != nil {
				ch <- errBlock("invalid cron schedule: " + err.Error())
				return
			}
			sched.mu.Lock()
			sched.tasks[id] = &ScheduledTask{
				ID: id, Schedule: in.Schedule,
				Command: cmd, Description: in.Description,
				CreatedAt: time.Now(),
			}
			sched.mu.Unlock()
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: fmt.Sprintf("Scheduled job %q (%s)", id, in.Schedule)}

		case "remove":
			sched.mu.Lock()
			delete(sched.tasks, in.ID)
			sched.mu.Unlock()
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: fmt.Sprintf("Removed job %q", in.ID)}

		case "list":
			sched.mu.RLock()
			b, _ := json.MarshalIndent(sched.tasks, "", "  ")
			sched.mu.RUnlock()
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(b)}
		}
	}()
	return ch, nil
}

func errBlock(msg string) *engine.ContentBlock {
	return &engine.ContentBlock{Type: engine.ContentTypeText, Text: msg, IsError: true}
}
