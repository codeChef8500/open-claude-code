package agent

import (
	"time"
)

// AgentStatus enumerates the lifecycle states of a sub-agent task.
type AgentStatus string

const (
	AgentStatusPending   AgentStatus = "pending"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusDone      AgentStatus = "done"
	AgentStatusFailed    AgentStatus = "failed"
	AgentStatusCancelled AgentStatus = "cancelled"
)

// AgentDefinitionSource indicates where an agent definition originated.
type AgentDefinitionSource string

const (
	SourceBuiltIn         AgentDefinitionSource = "builtin"
	SourceCustom          AgentDefinitionSource = "custom"
	SourcePlugin          AgentDefinitionSource = "plugin"
	SourceUserSettings    AgentDefinitionSource = "userSettings"
	SourceProjectSettings AgentDefinitionSource = "projectSettings"
	SourcePolicySettings  AgentDefinitionSource = "policySettings"
	SourceFlagSettings    AgentDefinitionSource = "flagSettings"
)

// IsolationMode controls agent execution isolation.
type IsolationMode string

const (
	IsolationNone     IsolationMode = ""
	IsolationWorktree IsolationMode = "worktree"
	IsolationRemote   IsolationMode = "remote"
)

// AgentPromptContext provides context for dynamic system prompt generation.
type AgentPromptContext struct {
	ToolNames []string
	Model     string
	WorkDir   string
	TeamName  string
	IsAsync   bool
	IsFork    bool
}

// AgentDefinition describes a sub-agent that can be spawned.
// Aligned with claude-code-main's BaseAgentDefinition / loadAgentsDir.ts.
type AgentDefinition struct {
	// ── Core identity ──────────────────────────────────────────────────
	AgentID   string                `json:"agent_id"`
	AgentType string                `json:"agent_type,omitempty"` // e.g. "general-purpose", "explore", "plan"
	Source    AgentDefinitionSource `json:"source,omitempty"`     // "builtin", "custom", "plugin"

	// ── Task description ───────────────────────────────────────────────
	Task      string `json:"task"`
	WhenToUse string `json:"when_to_use,omitempty"`

	// ── Execution configuration ────────────────────────────────────────
	WorkDir    string        `json:"work_dir"`
	MaxTurns   int           `json:"max_turns,omitempty"`
	Model      string        `json:"model,omitempty"`
	Effort     string        `json:"effort,omitempty"`     // "low","medium","high"
	Background bool          `json:"background,omitempty"` // default async execution
	Isolation  IsolationMode `json:"isolation,omitempty"`  // "worktree","remote",""

	// ── Tool control ───────────────────────────────────────────────────
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`
	Skills          []string `json:"skills,omitempty"`

	// ── Prompts ────────────────────────────────────────────────────────
	SystemPrompt           string `json:"system_prompt,omitempty"`
	InitialPrompt          string `json:"initial_prompt,omitempty"`
	OmitClaudeMd           bool   `json:"omit_claude_md,omitempty"`
	OmitGitStatus          bool   `json:"omit_git_status,omitempty"`
	CriticalSystemReminder string `json:"critical_system_reminder,omitempty"`

	// ── Dynamic system prompt generator (for builtin agents). ─────────
	// If set, called instead of using SystemPrompt directly.
	GetSystemPrompt func(ctx AgentPromptContext) string `json:"-"`

	// ── Permissions ────────────────────────────────────────────────────
	PermissionMode string `json:"permission_mode,omitempty"` // "acceptEdits","bypass","bubble" etc.

	// ── Memory ─────────────────────────────────────────────────────────
	Memory string `json:"memory,omitempty"` // "user","project","local"

	// ── MCP servers ────────────────────────────────────────────────────
	McpServers         []AgentMcpServerSpec `json:"mcp_servers,omitempty"`
	RequiredMcpServers []string             `json:"required_mcp_servers,omitempty"`

	// ── Hooks ──────────────────────────────────────────────────────────
	Hooks map[string][]HookCommand `json:"hooks,omitempty"`

	// ── Metadata ───────────────────────────────────────────────────────
	ParentID   string `json:"parent_id,omitempty"`
	TeamName   string `json:"team_name,omitempty"`
	Color      string `json:"color,omitempty"`       // ANSI colour for log output
	PluginName string `json:"plugin_name,omitempty"` // originating plugin name
}

// AgentMcpServerSpec defines an MCP server available to an agent.
type AgentMcpServerSpec struct {
	Name    string            `json:"name"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"` // SSE URL
}

// HookCommand defines an agent-level hook command.
type HookCommand struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // milliseconds
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

// IsSourceAdminTrusted returns true for sources that are admin-controlled.
// Plugin, built-in, and policySettings agents are admin-trusted.
func IsSourceAdminTrusted(source AgentDefinitionSource) bool {
	switch source {
	case SourceBuiltIn, SourcePlugin, SourcePolicySettings:
		return true
	default:
		return false
	}
}

// AgentDefinitionsResult holds the merged result of loading agents from all sources.
type AgentDefinitionsResult struct {
	ActiveAgents      []AgentDefinition
	AllAgents         []AgentDefinition
	FailedFiles       []FailedAgentFile
	AllowedAgentTypes []string // parsed from Agent(x,y) tool specs
}

// FailedAgentFile records an agent file that failed to load.
type FailedAgentFile struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}
