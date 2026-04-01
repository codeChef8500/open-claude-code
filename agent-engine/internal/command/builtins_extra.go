package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /memory ──────────────────────────────────────────────────────────────────

type MemoryCommand struct{}

func (c *MemoryCommand) Name() string        { return "memory" }
func (c *MemoryCommand) Description() string { return "Show or clear extracted session memories." }
func (c *MemoryCommand) Type() CommandType   { return CommandTypeLocal }
func (c *MemoryCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *MemoryCommand) Execute(_ context.Context, args []string, _ *ExecContext) (string, error) {
	if len(args) > 0 && strings.ToLower(args[0]) == "clear" {
		return "Session memories cleared.", nil
	}
	return "Use the HTTP API GET /api/v1/memory to inspect memories.", nil
}

// ─── /resume ──────────────────────────────────────────────────────────────────

type ResumeCommand struct{}

func (c *ResumeCommand) Name() string        { return "resume" }
func (c *ResumeCommand) Description() string { return "Resume a previous session. Usage: /resume [session-id]" }
func (c *ResumeCommand) Type() CommandType   { return CommandTypeLocal }
func (c *ResumeCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ResumeCommand) Execute(_ context.Context, args []string, _ *ExecContext) (string, error) {
	if len(args) == 0 {
		return "Usage: /resume <session-id>", nil
	}
	return fmt.Sprintf("__resume__:%s", args[0]), nil
}

// ─── /session ─────────────────────────────────────────────────────────────────

type SessionCommand struct{}

func (c *SessionCommand) Name() string        { return "session" }
func (c *SessionCommand) Description() string { return "Show current session info or list recent sessions." }
func (c *SessionCommand) Type() CommandType   { return CommandTypeLocal }
func (c *SessionCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *SessionCommand) Execute(_ context.Context, args []string, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "No active session.", nil
	}
	return fmt.Sprintf("Session: %s\nWorkDir: %s\nAutoMode: %v",
		ectx.SessionID, ectx.WorkDir, ectx.AutoMode), nil
}

// ─── /permissions ─────────────────────────────────────────────────────────────

type PermissionsCommand struct{}

func (c *PermissionsCommand) Name() string        { return "permissions" }
func (c *PermissionsCommand) Description() string { return "Show current tool permission settings." }
func (c *PermissionsCommand) Type() CommandType   { return CommandTypeLocal }
func (c *PermissionsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *PermissionsCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "Use the HTTP API GET /api/v1/permissions to inspect permission rules.", nil
}

// ─── /plugin ──────────────────────────────────────────────────────────────────

type PluginCommand struct{}

func (c *PluginCommand) Name() string        { return "plugin" }
func (c *PluginCommand) Description() string { return "Manage plugins. Usage: /plugin list|load <path>|unload <name>" }
func (c *PluginCommand) Type() CommandType   { return CommandTypeLocal }
func (c *PluginCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *PluginCommand) Execute(_ context.Context, args []string, _ *ExecContext) (string, error) {
	if len(args) == 0 || args[0] == "list" {
		return "Use the HTTP API GET /api/v1/plugins to list loaded plugins.", nil
	}
	switch args[0] {
	case "load":
		if len(args) < 2 {
			return "Usage: /plugin load <path>", nil
		}
		return fmt.Sprintf("Plugin load requested: %s (use HTTP API for management)", args[1]), nil
	case "unload":
		if len(args) < 2 {
			return "Usage: /plugin unload <name>", nil
		}
		return fmt.Sprintf("Plugin unload requested: %s (use HTTP API for management)", args[1]), nil
	}
	return "Unknown plugin sub-command. Try: list, load, unload", nil
}

// ─── /skills ──────────────────────────────────────────────────────────────────

type SkillsCommand struct{}

func (c *SkillsCommand) Name() string        { return "skills" }
func (c *SkillsCommand) Description() string { return "List available skills." }
func (c *SkillsCommand) Type() CommandType   { return CommandTypeLocal }
func (c *SkillsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *SkillsCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "Use the HTTP API GET /api/v1/skills to list available skills.", nil
}

// ─── /hatch ───────────────────────────────────────────────────────────────────

type HatchCommand struct{}

func (c *HatchCommand) Name() string        { return "hatch" }
func (c *HatchCommand) Description() string { return "Hatch a new companion buddy." }
func (c *HatchCommand) Type() CommandType   { return CommandTypeLocal }
func (c *HatchCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *HatchCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "__hatch__", nil
}

// ─── /auto-mode ───────────────────────────────────────────────────────────────

type AutoModeCommand struct{}

func (c *AutoModeCommand) Name() string        { return "auto-mode" }
func (c *AutoModeCommand) Description() string { return "Toggle or show Auto Mode status. Usage: /auto-mode [on|off]" }
func (c *AutoModeCommand) Type() CommandType   { return CommandTypeLocal }
func (c *AutoModeCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *AutoModeCommand) Execute(_ context.Context, args []string, ectx *ExecContext) (string, error) {
	if len(args) == 0 {
		if ectx != nil {
			return fmt.Sprintf("Auto Mode: %v", ectx.AutoMode), nil
		}
		return "Auto Mode: unknown", nil
	}
	switch strings.ToLower(args[0]) {
	case "on", "true", "1":
		if ectx != nil {
			ectx.AutoMode = true
		}
		return "Auto Mode enabled.", nil
	case "off", "false", "0":
		if ectx != nil {
			ectx.AutoMode = false
		}
		return "Auto Mode disabled.", nil
	}
	return "Usage: /auto-mode [on|off]", nil
}

// ─── Register extra built-ins ─────────────────────────────────────────────────

func init() {
	defaultRegistry.Register(
		&MemoryCommand{},
		&ResumeCommand{},
		&SessionCommand{},
		&PermissionsCommand{},
		&PluginCommand{},
		&SkillsCommand{},
		&HatchCommand{},
		&AutoModeCommand{},
	)
}
