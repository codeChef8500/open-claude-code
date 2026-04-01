package daemon

import "time"

// ScheduledTask describes a cron-driven agent task.
type ScheduledTask struct {
	ID         string    `json:"id"`
	CronExpr   string    `json:"cron_expr"`
	Task       string    `json:"task"`
	WorkDir    string    `json:"work_dir"`
	Durable    bool      `json:"durable"`
	Recurring  bool      `json:"recurring"`
	CreatedAt  time.Time `json:"created_at"`
	LastRunAt  time.Time `json:"last_run_at,omitempty"`
	NextRunAt  time.Time `json:"next_run_at,omitempty"`
	RunCount   int       `json:"run_count"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"` // zero = never
}

// TaskStatus describes the last execution outcome of a scheduled task.
type TaskStatus struct {
	TaskID    string    `json:"task_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Success   bool      `json:"success"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}
