package daemon

import "time"

// TaskStatus describes the last execution outcome of a scheduled task.
type TaskStatus struct {
	TaskID    string    `json:"task_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Success   bool      `json:"success"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}
