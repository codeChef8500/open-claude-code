package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// P1 Deep Implementations: /memory, /permissions, /feedback, /hooks,
// /stats, /agents, /tasks, /add-dir.
// These replace the shell stubs with structured data aligned to claude-code-main.
// ──────────────────────────────────────────────────────────────────────────────

// ─── /memory deep implementation ─────────────────────────────────────────────
// Aligned with claude-code-main commands/memory/memory.tsx.
// Provides file selector for editing CLAUDE.md memory files.

// MemoryViewData is the structured data for the memory TUI component.
type MemoryViewData struct {
	// Files is the list of discovered memory files.
	Files []MemoryFileEntry `json:"files"`
	// SelectedFile is a pre-selected file path (from args).
	SelectedFile string `json:"selected_file,omitempty"`
	// Editor is the configured editor command ($VISUAL or $EDITOR).
	Editor string `json:"editor,omitempty"`
	// Error if memory file discovery failed.
	Error string `json:"error,omitempty"`
}

// MemoryFileEntry represents a single memory file.
type MemoryFileEntry struct {
	Path     string `json:"path"`
	Label    string `json:"label"` // "project", "user", "enterprise"
	Exists   bool   `json:"exists"`
	Writable bool   `json:"writable"`
}

// DeepMemoryCommand replaces the basic MemoryCommand with full logic.
type DeepMemoryCommand struct{ BaseCommand }

func (c *DeepMemoryCommand) Name() string                  { return "memory" }
func (c *DeepMemoryCommand) Description() string           { return "Edit Claude memory files" }
func (c *DeepMemoryCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepMemoryCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepMemoryCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &MemoryViewData{}

	if len(args) > 0 {
		data.SelectedFile = strings.Join(args, " ")
	}

	// Detect editor.
	data.Editor = os.Getenv("VISUAL")
	if data.Editor == "" {
		data.Editor = os.Getenv("EDITOR")
	}
	if data.Editor == "" {
		if runtime.GOOS == "windows" {
			data.Editor = "notepad"
		} else {
			data.Editor = "vi"
		}
	}

	// Discover memory files.
	workDir := ""
	if ectx != nil {
		workDir = ectx.WorkDir
	}

	// Project-level CLAUDE.md
	if workDir != "" {
		projFile := filepath.Join(workDir, "CLAUDE.md")
		data.Files = append(data.Files, makeMemoryEntry(projFile, "project"))

		// Also check .claude/CLAUDE.md
		dotClaudeFile := filepath.Join(workDir, ".claude", "CLAUDE.md")
		if dotClaudeFile != projFile {
			data.Files = append(data.Files, makeMemoryEntry(dotClaudeFile, "project (.claude)"))
		}
	}

	// User-level ~/.claude/CLAUDE.md
	home, err := os.UserHomeDir()
	if err == nil {
		userFile := filepath.Join(home, ".claude", "CLAUDE.md")
		data.Files = append(data.Files, makeMemoryEntry(userFile, "user"))
	}

	return &InteractiveResult{
		Component: "memory",
		Data:      data,
	}, nil
}

func makeMemoryEntry(path, label string) MemoryFileEntry {
	entry := MemoryFileEntry{
		Path:  path,
		Label: label,
	}
	info, err := os.Stat(path)
	if err == nil {
		entry.Exists = true
		entry.Writable = info.Mode().Perm()&0200 != 0
	}
	return entry
}

// ─── /permissions deep implementation ────────────────────────────────────────
// Aligned with claude-code-main commands/permissions/permissions.tsx.
// Shows permission rules with the ability to retry denied commands.

// PermissionsViewData is the structured data for the permissions TUI component.
type PermissionsViewData struct {
	// AllowRules lists currently allowed tool patterns.
	AllowRules []PermissionRule `json:"allow_rules,omitempty"`
	// DenyRules lists currently denied tool patterns.
	DenyRules []PermissionRule `json:"deny_rules,omitempty"`
	// PermissionMode is the current permission mode.
	PermissionMode string `json:"permission_mode"`
	// Error if rule retrieval failed.
	Error string `json:"error,omitempty"`
}

// PermissionRule describes a single permission rule.
type PermissionRule struct {
	Tool    string `json:"tool"`
	Pattern string `json:"pattern,omitempty"`
	Source  string `json:"source"` // "user", "project", "session"
}

// DeepPermissionsCommand replaces the basic PermissionsCommand.
type DeepPermissionsCommand struct{ BaseCommand }

func (c *DeepPermissionsCommand) Name() string      { return "permissions" }
func (c *DeepPermissionsCommand) Aliases() []string { return []string{"allowed-tools"} }
func (c *DeepPermissionsCommand) Description() string {
	return "Manage allow & deny tool permission rules"
}
func (c *DeepPermissionsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepPermissionsCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepPermissionsCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &PermissionsViewData{}

	if ectx != nil {
		data.PermissionMode = ectx.PermissionMode

		// Load rules from config service.
		if ectx.Services != nil && ectx.Services.Config != nil {
			cfg := ectx.Services.Config
			if allow, ok := cfg.Get("allowedTools"); ok {
				if rules, ok := allow.([]interface{}); ok {
					for _, r := range rules {
						if s, ok := r.(string); ok {
							data.AllowRules = append(data.AllowRules, PermissionRule{
								Tool:   s,
								Source: cfg.GetSource("allowedTools"),
							})
						}
					}
				}
			}
			if deny, ok := cfg.Get("deniedTools"); ok {
				if rules, ok := deny.([]interface{}); ok {
					for _, r := range rules {
						if s, ok := r.(string); ok {
							data.DenyRules = append(data.DenyRules, PermissionRule{
								Tool:   s,
								Source: cfg.GetSource("deniedTools"),
							})
						}
					}
				}
			}
		}
	}

	return &InteractiveResult{
		Component: "permissions",
		Data:      data,
	}, nil
}

// ─── /feedback deep implementation ───────────────────────────────────────────
// Aligned with claude-code-main commands/feedback/feedback.tsx.
// Opens feedback form with conversation context.

// FeedbackViewData is the structured data for the feedback TUI component.
type FeedbackViewData struct {
	// InitialDescription is pre-filled from args.
	InitialDescription string `json:"initial_description,omitempty"`
	// SessionID for reference.
	SessionID string `json:"session_id,omitempty"`
	// TurnCount for context.
	TurnCount int `json:"turn_count,omitempty"`
	// Model used.
	Model string `json:"model,omitempty"`
}

// DeepFeedbackCommand replaces the basic FeedbackCommand.
type DeepFeedbackCommand struct{ BaseCommand }

func (c *DeepFeedbackCommand) Name() string                  { return "feedback" }
func (c *DeepFeedbackCommand) Description() string           { return "Send feedback about openclaude-go" }
func (c *DeepFeedbackCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepFeedbackCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepFeedbackCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &FeedbackViewData{}

	if len(args) > 0 {
		data.InitialDescription = strings.Join(args, " ")
	}
	if ectx != nil {
		data.SessionID = ectx.SessionID
		data.TurnCount = ectx.TurnCount
		data.Model = ectx.Model
	}

	return &InteractiveResult{
		Component: "feedback",
		Data:      data,
	}, nil
}

// ─── /hooks deep implementation ──────────────────────────────────────────────
// Aligned with claude-code-main commands/hooks/hooks.tsx.
// Shows hook configurations with available tool names.

// HooksViewData is the structured data for the hooks TUI component.
type HooksViewData struct {
	// ToolNames lists available tools that can have hooks.
	ToolNames []string `json:"tool_names,omitempty"`
	// Hooks maps hook event type to hook configurations.
	Hooks map[string][]HooksConfigEntry `json:"hooks,omitempty"`
	// Error if hook loading failed.
	Error string `json:"error,omitempty"`
}

// HooksConfigEntry describes a configured hook for the hooks TUI view.
type HooksConfigEntry struct {
	Command   string `json:"command"`
	EventType string `json:"event_type"` // "pre", "post"
	ToolName  string `json:"tool_name,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
}

// DeepHooksCommand replaces the basic HooksCommand.
type DeepHooksCommand struct{ BaseCommand }

func (c *DeepHooksCommand) Name() string                  { return "hooks" }
func (c *DeepHooksCommand) Description() string           { return "View hook configurations for tool events" }
func (c *DeepHooksCommand) IsImmediate() bool             { return true }
func (c *DeepHooksCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepHooksCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepHooksCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &HooksViewData{
		Hooks: make(map[string][]HooksConfigEntry),
	}

	// Populate tool names from config.
	if ectx != nil && ectx.Services != nil && ectx.Services.Config != nil {
		cfg := ectx.Services.Config
		// Get hooks config.
		if hooksVal, ok := cfg.Get("hooks"); ok {
			if hooksMap, ok := hooksVal.(map[string]interface{}); ok {
				for eventType, entries := range hooksMap {
					if entryList, ok := entries.([]interface{}); ok {
						for _, e := range entryList {
							if em, ok := e.(map[string]interface{}); ok {
								he := HooksConfigEntry{EventType: eventType}
								if cmd, ok := em["command"].(string); ok {
									he.Command = cmd
								}
								if tn, ok := em["tool_name"].(string); ok {
									he.ToolName = tn
								}
								data.Hooks[eventType] = append(data.Hooks[eventType], he)
							}
						}
					}
				}
			}
		}
	}

	return &InteractiveResult{
		Component: "hooks",
		Data:      data,
	}, nil
}

// ─── /stats deep implementation ──────────────────────────────────────────────
// Aligned with claude-code-main commands/stats/stats.tsx.
// Shows session statistics including tokens, cost, time.

// StatsViewData is the structured data for the stats TUI component.
type StatsViewData struct {
	SessionID    string `json:"session_id"`
	Model        string `json:"model"`
	TurnCount    int    `json:"turn_count"`
	TotalTokens  int    `json:"total_tokens"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
	CacheReads   int    `json:"cache_reads,omitempty"`
	CacheWrites  int    `json:"cache_writes,omitempty"`
	CostUSD      string `json:"cost_usd"`
	Duration     string `json:"duration,omitempty"`
}

// DeepStatsCommand replaces the basic StatsCommand.
type DeepStatsCommand struct{ BaseCommand }

func (c *DeepStatsCommand) Name() string                  { return "stats" }
func (c *DeepStatsCommand) Description() string           { return "Show session statistics" }
func (c *DeepStatsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepStatsCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepStatsCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &StatsViewData{}

	if ectx != nil {
		data.SessionID = ectx.SessionID
		data.Model = ectx.Model
		data.TurnCount = ectx.TurnCount
		data.TotalTokens = ectx.TotalTokens
		data.CostUSD = fmt.Sprintf("$%.4f", ectx.CostUSD)
	}

	return &InteractiveResult{
		Component: "stats",
		Data:      data,
	}, nil
}

// ─── /agents deep implementation ─────────────────────────────────────────────
// Aligned with claude-code-main commands/agents/agents.tsx.
// Shows agent configurations and available tools.

// AgentsViewData is the structured data for the agents TUI component.
type AgentsViewData struct {
	// AvailableTools lists tools the user can use with agents.
	AvailableTools []string `json:"available_tools,omitempty"`
	// ActiveAgents lists currently active agent instances.
	ActiveAgents []AgentEntry `json:"active_agents,omitempty"`
}

// AgentEntry describes an active or configured agent.
type AgentEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"` // "fork", "teammate", "async"
	Status string `json:"status"`
}

// DeepAgentsCommand replaces the basic AgentsCommand.
type DeepAgentsCommand struct{ BaseCommand }

func (c *DeepAgentsCommand) Name() string                  { return "agents" }
func (c *DeepAgentsCommand) Description() string           { return "Manage agent configurations" }
func (c *DeepAgentsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepAgentsCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepAgentsCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &AgentsViewData{}

	// Retrieve active tasks (agents) from TaskService.
	if ectx != nil && ectx.Services != nil && ectx.Services.Task != nil {
		tasks := ectx.Services.Task.ListTasks()
		for _, t := range tasks {
			data.ActiveAgents = append(data.ActiveAgents, AgentEntry{
				ID:     t.ID,
				Name:   t.Title,
				Type:   "async",
				Status: t.Status,
			})
		}
	}

	return &InteractiveResult{
		Component: "agents",
		Data:      data,
	}, nil
}

// ─── /tasks deep implementation ──────────────────────────────────────────────
// Aligned with claude-code-main commands/tasks/tasks.tsx.
// Shows background tasks dialog.

// TasksViewData is the structured data for the tasks TUI component.
type TasksViewData struct {
	// Tasks lists all background tasks.
	Tasks []TaskViewEntry `json:"tasks,omitempty"`
}

// TaskViewEntry is a display-friendly task entry.
type TaskViewEntry struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// DeepTasksCommand replaces the basic TasksCommand.
type DeepTasksCommand struct{ BaseCommand }

func (c *DeepTasksCommand) Name() string                  { return "tasks" }
func (c *DeepTasksCommand) Aliases() []string             { return []string{"bashes"} }
func (c *DeepTasksCommand) Description() string           { return "List and manage background tasks" }
func (c *DeepTasksCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepTasksCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepTasksCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &TasksViewData{}

	if ectx != nil && ectx.Services != nil && ectx.Services.Task != nil {
		tasks := ectx.Services.Task.ListTasks()
		for _, t := range tasks {
			data.Tasks = append(data.Tasks, TaskViewEntry{
				ID:        t.ID,
				AgentID:   t.AgentID,
				Title:     t.Title,
				Status:    t.Status,
				CreatedAt: formatTimestampP1(t.CreatedAt),
			})
		}
	}

	return &InteractiveResult{
		Component: "tasks",
		Data:      data,
	}, nil
}

// ─── /add-dir deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/add-dir/add-dir.tsx.
// Validates path and adds directory to workspace.

// AddDirViewData is the structured data for the add-dir TUI component.
type AddDirViewData struct {
	// Path is the directory path to add.
	Path string `json:"path,omitempty"`
	// ResolvedPath is the absolute resolved path.
	ResolvedPath string `json:"resolved_path,omitempty"`
	// IsValid indicates whether the path is a valid directory.
	IsValid bool `json:"is_valid"`
	// Error message if validation failed.
	Error string `json:"error,omitempty"`
	// CurrentDirs lists the current workspace directories.
	CurrentDirs []string `json:"current_dirs,omitempty"`
}

// DeepAddDirCommand replaces the basic AddDirCommand.
type DeepAddDirCommand struct{ BaseCommand }

func (c *DeepAddDirCommand) Name() string                  { return "add-dir" }
func (c *DeepAddDirCommand) Description() string           { return "Add a new working directory" }
func (c *DeepAddDirCommand) ArgumentHint() string          { return "<path>" }
func (c *DeepAddDirCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepAddDirCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepAddDirCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &AddDirViewData{}

	if len(args) > 0 {
		data.Path = strings.Join(args, " ")

		// Resolve the path.
		resolved := data.Path
		if !filepath.IsAbs(resolved) && ectx != nil {
			resolved = filepath.Join(ectx.WorkDir, resolved)
		}
		resolved = filepath.Clean(resolved)
		data.ResolvedPath = resolved

		// Validate the path is a directory.
		info, err := os.Stat(resolved)
		if err != nil {
			data.Error = fmt.Sprintf("Path does not exist: %s", resolved)
		} else if !info.IsDir() {
			data.Error = fmt.Sprintf("Not a directory: %s", resolved)
		} else {
			data.IsValid = true
		}
	}

	// Report current workspace dirs.
	if ectx != nil {
		data.CurrentDirs = append(data.CurrentDirs, ectx.WorkDir)
	}

	return &InteractiveResult{
		Component: "add-dir",
		Data:      data,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// formatTimestampP1 is a local timestamp formatter to avoid collision with
// the one in session_impl.go (different package init order is fine, but
// keeping names unique avoids confusion).
func formatTimestampP1(ts int64) string {
	return formatTimestamp(ts)
}

// ──────────────────────────────────────────────────────────────────────────────
// Register P1 deep commands, replacing stubs.
// ──────────────────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepMemoryCommand{},
		&DeepPermissionsCommand{},
		&DeepFeedbackCommand{},
		&DeepHooksCommand{},
		&DeepStatsCommand{},
		&DeepAgentsCommand{},
		&DeepTasksCommand{},
		&DeepAddDirCommand{},
	)
}
