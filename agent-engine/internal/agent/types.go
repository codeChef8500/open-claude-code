package agent

import "time"

// AgentStatus enumerates the lifecycle states of a sub-agent task.
type AgentStatus string

const (
	AgentStatusPending   AgentStatus = "pending"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusDone      AgentStatus = "done"
	AgentStatusFailed    AgentStatus = "failed"
	AgentStatusCancelled AgentStatus = "cancelled"
)

// AgentDefinition describes a sub-agent that can be spawned.
type AgentDefinition struct {
	AgentID      string   `json:"agent_id"`
	Task         string   `json:"task"`
	WorkDir      string   `json:"work_dir"`
	MaxTurns     int      `json:"max_turns,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	ParentID     string   `json:"parent_id,omitempty"`
	TeamName     string   `json:"team_name,omitempty"`
	Color        string   `json:"color,omitempty"` // ANSI colour for log output
}

// AgentTask is the runtime record of a spawned sub-agent.
type AgentTask struct {
	Definition AgentDefinition `json:"definition"`
	Status     AgentStatus     `json:"status"`
	StartedAt  time.Time       `json:"started_at,omitempty"`
	FinishedAt time.Time       `json:"finished_at,omitempty"`
	Output     string          `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// AgentMessage is a message routed between agents via channel queues.
type AgentMessage struct {
	FromAgentID string      `json:"from_agent_id"`
	ToAgentID   string      `json:"to_agent_id"`
	Content     interface{} `json:"content"`
}
