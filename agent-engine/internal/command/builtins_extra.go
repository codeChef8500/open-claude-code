package command

import (
	"context"
	"fmt"
	"strings"
)

// MemoryCommand shell removed — replaced by DeepMemoryCommand in commands_deep_p1.go.

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

// SessionCommand shell removed — replaced by DeepSessionCommand in session_impl.go.

// PermissionsCommand shell removed — replaced by DeepPermissionsCommand in commands_deep_p1.go.

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
		&ResumeCommand{},
		&PluginCommand{},
		&SkillsCommand{},
		&BuddyCommand{},
		&AutoModeCommand{},
	)
}
