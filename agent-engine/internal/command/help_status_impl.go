package command

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// Deep /help and /status implementations.
// Aligned with claude-code-main HelpV2 (tabbed: general/commands/custom-commands)
// and status.tsx (Settings component with "Status" default tab).
// ──────────────────────────────────────────────────────────────────────────────

// ─── /help deep implementation ───────────────────────────────────────────────
// Aligned with claude-code-main components/HelpV2/HelpV2.tsx.

// HelpViewData is the structured data for the help TUI component.
type HelpViewData struct {
	// BuiltinCommands are the default built-in commands.
	BuiltinCommands []HelpEntry `json:"builtin_commands"`
	// CustomCommands are plugin/skill/workflow/MCP commands.
	CustomCommands []HelpEntry `json:"custom_commands"`
	// Version is the engine version string.
	Version string `json:"version"`
	// FallbackText is the plain-text help output for non-interactive contexts.
	FallbackText string `json:"fallback_text,omitempty"`
}

// HelpEntry is a display-friendly command entry for the help panel.
type HelpEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgHint     string `json:"arg_hint,omitempty"`
	Source      string `json:"source,omitempty"` // builtin, plugin, skill, mcp, workflow
	IsHidden    bool   `json:"is_hidden,omitempty"`
}

// DeepHelpCommand replaces the basic HelpCommand with grouped display.
type DeepHelpCommand struct{ BaseCommand }

func (c *DeepHelpCommand) Name() string                  { return "help" }
func (c *DeepHelpCommand) Description() string           { return "Show available slash commands." }
func (c *DeepHelpCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepHelpCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepHelpCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &HelpViewData{
		Version: "0.1.0",
	}

	if ectx == nil {
		data.FallbackText = "Available commands: (no context)"
		return &InteractiveResult{Component: "help", Data: data}, nil
	}

	// Collect all visible commands, split into builtin vs custom.
	builtinNames := builtinCommandNames()
	for _, cmd := range defaultRegistry.VisibleFor(ectx, AvailabilityConsole) {
		entry := HelpEntry{
			Name:        cmd.Name(),
			Description: FormatDescriptionWithSource(cmd),
			ArgHint:     cmd.ArgumentHint(),
		}

		if _, ok := builtinNames[cmd.Name()]; ok {
			entry.Source = "builtin"
			data.BuiltinCommands = append(data.BuiltinCommands, entry)
		} else {
			// Determine source from command metadata.
			entry.Source = commandSource(cmd)
			data.CustomCommands = append(data.CustomCommands, entry)
		}
	}

	// Build fallback text for non-interactive contexts.
	data.FallbackText = buildHelpText(data)

	return &InteractiveResult{
		Component: "help",
		Data:      data,
	}, nil
}

// builtinCommandNames returns the set of names that are considered built-in.
func builtinCommandNames() map[string]struct{} {
	names := map[string]struct{}{
		"help": {}, "clear": {}, "compact": {}, "status": {}, "model": {},
		"version": {}, "quit": {}, "exit": {},
		"commit": {}, "review": {}, "security-review": {}, "init": {},
		"cost": {}, "config": {}, "mcp": {}, "doctor": {}, "bug-report": {},
		"login": {}, "logout": {}, "usage": {},
		"theme": {}, "color": {}, "copy": {},
		"memory": {}, "resume": {}, "session": {}, "permissions": {},
		"plugin": {}, "skills": {}, "buddy": {}, "auto-mode": {},
		"context": {}, "diff": {}, "rewind": {}, "branch": {},
		"pr-comments": {}, "commit-push-pr": {},
		"add-dir": {}, "hooks": {}, "feedback": {}, "stats": {},
		"advisor": {}, "tag": {}, "desktop": {}, "privacy-settings": {},
		"upgrade": {}, "btw": {}, "release-notes": {}, "terminal-setup": {},
		"statusline": {},
		"mobile":     {}, "chrome": {}, "ide": {},
		"sandbox-toggle": {}, "rate-limit-options": {},
		"install-github-app": {}, "install-slack-app": {},
		"remote-env": {}, "remote-setup": {},
		"thinkback": {}, "thinkback-play": {},
		"reload-plugins": {}, "bridge-kick": {},
		"files": {}, "insights": {}, "init-verifiers": {}, "heapdump": {},
		"agents": {}, "tasks": {}, "workflow": {},
	}
	return names
}

// commandSource determines the source label for a non-builtin command.
func commandSource(cmd Command) string {
	return string(cmd.Source())
}

// buildHelpText produces a flat text listing from HelpViewData.
func buildHelpText(data *HelpViewData) string {
	var lines []string
	lines = append(lines, "Available commands:")

	if len(data.BuiltinCommands) > 0 {
		lines = append(lines, "")
		lines = append(lines, "  Built-in commands:")
		for _, e := range data.BuiltinCommands {
			if e.ArgHint != "" {
				lines = append(lines, fmt.Sprintf("    /%s %s — %s", e.Name, e.ArgHint, e.Description))
			} else {
				lines = append(lines, fmt.Sprintf("    /%s — %s", e.Name, e.Description))
			}
		}
	}

	if len(data.CustomCommands) > 0 {
		lines = append(lines, "")
		lines = append(lines, "  Custom commands:")
		for _, e := range data.CustomCommands {
			if e.ArgHint != "" {
				lines = append(lines, fmt.Sprintf("    /%s %s — %s", e.Name, e.ArgHint, e.Description))
			} else {
				lines = append(lines, fmt.Sprintf("    /%s — %s", e.Name, e.Description))
			}
		}
	}

	return strings.Join(lines, "\n")
}

// ─── /status deep implementation ─────────────────────────────────────────────
// Aligned with claude-code-main commands/status/status.tsx.

// StatusViewDataV2 is the structured data for the status TUI component.
// Named V2 to avoid conflict with SessionViewData in session_impl.go.
type StatusViewDataV2 struct {
	SessionID      string `json:"session_id"`
	WorkDir        string `json:"work_dir"`
	Model          string `json:"model"`
	TurnCount      int    `json:"turn_count"`
	TotalTokens    int    `json:"total_tokens"`
	CostUSD        string `json:"cost_usd"`
	PlanMode       bool   `json:"plan_mode"`
	FastMode       bool   `json:"fast_mode"`
	AutoMode       bool   `json:"auto_mode"`
	EffortLevel    string `json:"effort_level"`
	PermissionMode string `json:"permission_mode"`
	MCPServers     int    `json:"mcp_servers"`
	SandboxMode    string `json:"sandbox_mode,omitempty"`
	ActiveTools    int    `json:"active_tools,omitempty"`
	ActiveAgents   int    `json:"active_agents,omitempty"`
	// FallbackText for non-interactive contexts.
	FallbackText string `json:"fallback_text,omitempty"`
}

// DeepStatusCommand replaces the basic StatusCommand with detailed status.
type DeepStatusCommand struct{ BaseCommand }

func (c *DeepStatusCommand) Name() string                  { return "status" }
func (c *DeepStatusCommand) Description() string           { return "Show engine status." }
func (c *DeepStatusCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepStatusCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepStatusCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &StatusViewDataV2{}

	if ectx != nil {
		data.SessionID = ectx.SessionID
		data.WorkDir = ectx.WorkDir
		data.Model = ectx.Model
		data.TurnCount = ectx.TurnCount
		data.TotalTokens = ectx.TotalTokens
		data.CostUSD = fmt.Sprintf("$%.4f", ectx.CostUSD)
		data.PlanMode = ectx.PlanModeActive
		data.FastMode = ectx.FastMode
		data.AutoMode = ectx.AutoMode
		data.EffortLevel = ectx.EffortLevel
		data.PermissionMode = ectx.PermissionMode
		data.MCPServers = len(ectx.ActiveMCPServers)
	}

	// Build fallback text.
	data.FallbackText = buildStatusText(data)

	return &InteractiveResult{
		Component: "status",
		Data:      data,
	}, nil
}

// buildStatusText produces a flat text representation for non-interactive use.
func buildStatusText(data *StatusViewDataV2) string {
	lines := []string{
		fmt.Sprintf("Session: %s", data.SessionID),
		fmt.Sprintf("WorkDir: %s", data.WorkDir),
		fmt.Sprintf("Model:   %s", data.Model),
		fmt.Sprintf("Turns:   %d", data.TurnCount),
		fmt.Sprintf("Tokens:  %d", data.TotalTokens),
		fmt.Sprintf("Cost:    %s", data.CostUSD),
	}
	if data.PlanMode {
		lines = append(lines, "Mode:    plan")
	}
	if data.FastMode {
		lines = append(lines, "Fast:    on")
	}
	if data.AutoMode {
		lines = append(lines, "Auto:    on")
	}
	if data.EffortLevel != "" {
		lines = append(lines, fmt.Sprintf("Effort:  %s", data.EffortLevel))
	}
	if data.PermissionMode != "" {
		lines = append(lines, fmt.Sprintf("Perms:   %s", data.PermissionMode))
	}
	if data.MCPServers > 0 {
		lines = append(lines, fmt.Sprintf("MCP:     %d servers", data.MCPServers))
	}
	return strings.Join(lines, "\n")
}

// ──────────────────────────────────────────────────────────────────────────────
// Register deep help/status, replacing basic stubs.
// ──────────────────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepHelpCommand{},
		&DeepStatusCommand{},
	)
}
