package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /memory ──────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/memory/index.ts (local-jsx).

type MemoryCommand struct{ BaseCommand }

func (c *MemoryCommand) Name() string                  { return "memory" }
func (c *MemoryCommand) Description() string           { return "Edit Claude memory files" }
func (c *MemoryCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *MemoryCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *MemoryCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "memory"}, nil
}

// ─── /resume ──────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/resume/index.ts (local-jsx).

type ResumeCommand struct{ BaseCommand }

func (c *ResumeCommand) Name() string                  { return "resume" }
func (c *ResumeCommand) Aliases() []string             { return []string{"continue"} }
func (c *ResumeCommand) Description() string           { return "Resume a previous conversation" }
func (c *ResumeCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ResumeCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ResumeCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	sessionID := ""
	if len(args) > 0 {
		sessionID = args[0]
	}
	return &InteractiveResult{
		Component: "resume",
		Data:      map[string]interface{}{"sessionID": sessionID},
	}, nil
}

// ─── /session ─────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/session/index.ts (local-jsx).

type SessionCommand struct{ BaseCommand }

func (c *SessionCommand) Name() string                  { return "session" }
func (c *SessionCommand) Aliases() []string             { return []string{"remote"} }
func (c *SessionCommand) Description() string           { return "Show session info and remote URL" }
func (c *SessionCommand) Type() CommandType             { return CommandTypeLocal }
func (c *SessionCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *SessionCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "No active session.", nil
	}
	lines := []string{
		fmt.Sprintf("Session: %s", ectx.SessionID),
		fmt.Sprintf("WorkDir: %s", ectx.WorkDir),
		fmt.Sprintf("Model:   %s", ectx.Model),
		fmt.Sprintf("Turns:   %d", ectx.TurnCount),
	}
	if ectx.PlanModeActive {
		lines = append(lines, "Mode:    plan")
	}
	if ectx.FastMode {
		lines = append(lines, "Fast:    on")
	}
	if ectx.AutoMode {
		lines = append(lines, "Auto:    on")
	}
	return strings.Join(lines, "\n"), nil
}

// ─── /permissions ─────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/permissions/index.ts (local-jsx).

type PermissionsCommand struct{ BaseCommand }

func (c *PermissionsCommand) Name() string                  { return "permissions" }
func (c *PermissionsCommand) Aliases() []string             { return []string{"allowed-tools"} }
func (c *PermissionsCommand) Description() string           { return "Manage allow & deny tool permission rules" }
func (c *PermissionsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *PermissionsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *PermissionsCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "permissions"}, nil
}

// ─── /plugin ──────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/plugin/index.tsx (local-jsx).

type PluginCommand struct{ BaseCommand }

func (c *PluginCommand) Name() string                  { return "plugin" }
func (c *PluginCommand) Description() string           { return "Manage plugins" }
func (c *PluginCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *PluginCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *PluginCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	return &InteractiveResult{
		Component: "plugin",
		Data:      map[string]interface{}{"subcommand": sub, "args": args},
	}, nil
}

// ─── /skills ──────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/skills/index.ts (local-jsx).

type SkillsCommand struct{ BaseCommand }

func (c *SkillsCommand) Name() string                  { return "skills" }
func (c *SkillsCommand) Description() string           { return "List available skills" }
func (c *SkillsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *SkillsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *SkillsCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "skills"}, nil
}

// ─── /hatch ───────────────────────────────────────────────────────────────────

type HatchCommand struct{ BaseCommand }

func (c *HatchCommand) Name() string                  { return "hatch" }
func (c *HatchCommand) Description() string           { return "Hatch a new companion buddy" }
func (c *HatchCommand) Type() CommandType             { return CommandTypeLocal }
func (c *HatchCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *HatchCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "__hatch__", nil
}

// ─── /auto-mode ───────────────────────────────────────────────────────────────

type AutoModeCommand struct{ BaseCommand }

func (c *AutoModeCommand) Name() string         { return "auto-mode" }
func (c *AutoModeCommand) ArgumentHint() string { return "[on|off]" }
func (c *AutoModeCommand) Description() string {
	return "Toggle or show Auto Mode status"
}
func (c *AutoModeCommand) Type() CommandType             { return CommandTypeLocal }
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
