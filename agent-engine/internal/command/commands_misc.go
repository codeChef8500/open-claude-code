package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /add-dir ────────────────────────────────────────────────────────────────

// AddDirCommand adds a new working directory.
// Aligned with claude-code-main commands/add-dir/index.ts (local-jsx).
type AddDirCommand struct{ BaseCommand }

func (c *AddDirCommand) Name() string                  { return "add-dir" }
func (c *AddDirCommand) Description() string           { return "Add a new working directory" }
func (c *AddDirCommand) ArgumentHint() string          { return "<path>" }
func (c *AddDirCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *AddDirCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *AddDirCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	path := ""
	if len(args) > 0 {
		path = strings.Join(args, " ")
	}
	return &InteractiveResult{
		Component: "add-dir",
		Data:      map[string]interface{}{"path": path},
	}, nil
}

// ─── /hooks ──────────────────────────────────────────────────────────────────

// HooksCommand views hook configurations for tool events.
// Aligned with claude-code-main commands/hooks/index.ts (local-jsx, immediate).
type HooksCommand struct{ BaseCommand }

func (c *HooksCommand) Name() string                  { return "hooks" }
func (c *HooksCommand) Description() string           { return "View hook configurations for tool events" }
func (c *HooksCommand) IsImmediate() bool             { return true }
func (c *HooksCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *HooksCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *HooksCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "hooks"}, nil
}

// ─── /feedback ───────────────────────────────────────────────────────────────

// FeedbackCommand opens the feedback form.
// Aligned with claude-code-main commands/feedback/index.ts (local-jsx).
type FeedbackCommand struct{ BaseCommand }

func (c *FeedbackCommand) Name() string                  { return "feedback" }
func (c *FeedbackCommand) Description() string           { return "Send feedback about openclaude-go" }
func (c *FeedbackCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *FeedbackCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *FeedbackCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "feedback"}, nil
}

// ─── /stats ──────────────────────────────────────────────────────────────────

// StatsCommand shows session statistics.
// Aligned with claude-code-main commands/stats/index.ts (local-jsx).
type StatsCommand struct{ BaseCommand }

func (c *StatsCommand) Name() string                  { return "stats" }
func (c *StatsCommand) Description() string           { return "Show session statistics" }
func (c *StatsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *StatsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *StatsCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "stats"}, nil
}

// ─── /advisor ────────────────────────────────────────────────────────────────

// AdvisorCommand configures the advisor model.
// Aligned with claude-code-main commands/advisor.ts (local).
type AdvisorCommand struct{ BaseCommand }

func (c *AdvisorCommand) Name() string                  { return "advisor" }
func (c *AdvisorCommand) Description() string           { return "Configure the advisor model" }
func (c *AdvisorCommand) ArgumentHint() string          { return "[<model>|off]" }
func (c *AdvisorCommand) Type() CommandType             { return CommandTypeLocal }
func (c *AdvisorCommand) IsHidden() bool                { return true } // hidden by default, shown when advisor is available
func (c *AdvisorCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *AdvisorCommand) Execute(_ context.Context, args []string, _ *ExecContext) (string, error) {
	if len(args) == 0 {
		return "Advisor: not set\nUse \"/advisor <model>\" to enable (e.g. \"/advisor opus\").", nil
	}
	arg := strings.ToLower(strings.TrimSpace(args[0]))
	if arg == "unset" || arg == "off" {
		return "Advisor disabled.", nil
	}
	return fmt.Sprintf("Advisor set to %s.", arg), nil
}

// ─── /tag ────────────────────────────────────────────────────────────────────

// TagCommand toggles a searchable tag on the current session.
// Aligned with claude-code-main commands/tag/index.ts (local-jsx, ant-only).
type TagCommand struct{ BaseCommand }

func (c *TagCommand) Name() string                  { return "tag" }
func (c *TagCommand) Description() string           { return "Toggle a searchable tag on the current session" }
func (c *TagCommand) ArgumentHint() string          { return "<tag-name>" }
func (c *TagCommand) IsHidden() bool                { return true } // ant-only
func (c *TagCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *TagCommand) IsEnabled(_ *ExecContext) bool { return false } // ant-only, disabled by default
func (c *TagCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	tag := ""
	if len(args) > 0 {
		tag = strings.Join(args, " ")
	}
	return &InteractiveResult{
		Component: "tag",
		Data:      map[string]interface{}{"tag": tag},
	}, nil
}

// ─── /usage ──────────────────────────────────────────────────────────────────

// UsageCommand shows plan usage limits.
// Aligned with claude-code-main commands/usage/index.ts (local-jsx, claude-ai only).
type UsageCommand struct{ BaseCommand }

func (c *UsageCommand) Name() string        { return "usage" }
func (c *UsageCommand) Description() string { return "Show plan usage limits" }
func (c *UsageCommand) Availability() []CommandAvailability {
	return []CommandAvailability{AvailabilityClaudeAI}
}
func (c *UsageCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *UsageCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *UsageCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "usage"}, nil
}

// ─── /desktop ────────────────────────────────────────────────────────────────

// DesktopCommand manages desktop app settings.
// Aligned with claude-code-main commands/desktop/index.ts (local-jsx).
type DesktopCommand struct{ BaseCommand }

func (c *DesktopCommand) Name() string                  { return "desktop" }
func (c *DesktopCommand) Description() string           { return "Manage desktop app settings" }
func (c *DesktopCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DesktopCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *DesktopCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "desktop"}, nil
}

// ─── /privacy-settings ───────────────────────────────────────────────────────

// PrivacySettingsCommand configures privacy settings.
// Aligned with claude-code-main commands/privacy-settings/index.ts (local-jsx).
type PrivacySettingsCommand struct{ BaseCommand }

func (c *PrivacySettingsCommand) Name() string                  { return "privacy-settings" }
func (c *PrivacySettingsCommand) Aliases() []string             { return []string{"privacy"} }
func (c *PrivacySettingsCommand) Description() string           { return "Configure privacy settings" }
func (c *PrivacySettingsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *PrivacySettingsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *PrivacySettingsCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "privacy-settings"}, nil
}

// ─── /upgrade ────────────────────────────────────────────────────────────────

// UpgradeCommand upgrades openclaude-go to the latest version.
// Aligned with claude-code-main commands/upgrade/index.ts (local-jsx).
type UpgradeCommand struct{ BaseCommand }

func (c *UpgradeCommand) Name() string                  { return "upgrade" }
func (c *UpgradeCommand) Aliases() []string             { return []string{"update"} }
func (c *UpgradeCommand) Description() string           { return "Upgrade to the latest version" }
func (c *UpgradeCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *UpgradeCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *UpgradeCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "upgrade"}, nil
}

// ─── /reload-plugins ─────────────────────────────────────────────────────────

// ReloadPluginsCommand reloads all plugins.
// Aligned with claude-code-main commands/reload-plugins/index.ts (local-jsx).
type ReloadPluginsCommand struct{ BaseCommand }

func (c *ReloadPluginsCommand) Name() string                  { return "reload-plugins" }
func (c *ReloadPluginsCommand) Description() string           { return "Reload all plugins" }
func (c *ReloadPluginsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ReloadPluginsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ReloadPluginsCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "reload-plugins"}, nil
}

// ─── /bridge ─────────────────────────────────────────────────────────────────

// BridgeCommand manages bridge connections.
// Aligned with claude-code-main commands/bridge/index.ts (local-jsx).
type BridgeCommand struct{ BaseCommand }

func (c *BridgeCommand) Name() string                  { return "bridge" }
func (c *BridgeCommand) Description() string           { return "Manage bridge connections" }
func (c *BridgeCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *BridgeCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *BridgeCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "bridge"}, nil
}

// ─── /bridge-kick ────────────────────────────────────────────────────────────

// BridgeKickCommand kicks a bridge peer.
// Aligned with claude-code-main commands/bridge-kick.ts (prompt).
type BridgeKickCommand struct{ BasePromptCommand }

func (c *BridgeKickCommand) Name() string                  { return "bridge-kick" }
func (c *BridgeKickCommand) Description() string           { return "Kick a bridge peer" }
func (c *BridgeKickCommand) IsHidden() bool                { return true }
func (c *BridgeKickCommand) Type() CommandType             { return CommandTypePrompt }
func (c *BridgeKickCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *BridgeKickCommand) PromptContent(_ []string, _ *ExecContext) (string, error) {
	return "Kick the bridge peer from this session.", nil
}

// ─── /btw ────────────────────────────────────────────────────────────────────

// BtwCommand sends a side message to the model.
// Aligned with claude-code-main commands/btw/index.ts (local-jsx).
type BtwCommand struct{ BaseCommand }

func (c *BtwCommand) Name() string                  { return "btw" }
func (c *BtwCommand) Description() string           { return "Send a side message to the model" }
func (c *BtwCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *BtwCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *BtwCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	msg := ""
	if len(args) > 0 {
		msg = strings.Join(args, " ")
	}
	return &InteractiveResult{
		Component: "btw",
		Data:      map[string]interface{}{"message": msg},
	}, nil
}

// ─── /passes ─────────────────────────────────────────────────────────────────

// PassesCommand shows active passes.
// Aligned with claude-code-main commands/passes/index.ts (local-jsx).
type PassesCommand struct{ BaseCommand }

func (c *PassesCommand) Name() string                  { return "passes" }
func (c *PassesCommand) Description() string           { return "Show active passes" }
func (c *PassesCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *PassesCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *PassesCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "passes"}, nil
}

// ─── /release-notes ──────────────────────────────────────────────────────────

// ReleaseNotesCommand shows release notes.
// Aligned with claude-code-main commands/release-notes/index.ts (local-jsx).
type ReleaseNotesCommand struct{ BaseCommand }

func (c *ReleaseNotesCommand) Name() string                  { return "release-notes" }
func (c *ReleaseNotesCommand) Aliases() []string             { return []string{"changelog"} }
func (c *ReleaseNotesCommand) Description() string           { return "Show release notes" }
func (c *ReleaseNotesCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ReleaseNotesCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ReleaseNotesCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "release-notes"}, nil
}

// ─── /terminal-setup ─────────────────────────────────────────────────────────

// TerminalSetupCommand configures terminal settings.
// Aligned with claude-code-main commands/terminalSetup/index.ts (local-jsx).
type TerminalSetupCommand struct{ BaseCommand }

func (c *TerminalSetupCommand) Name() string                  { return "terminal-setup" }
func (c *TerminalSetupCommand) Aliases() []string             { return []string{"terminalsetup"} }
func (c *TerminalSetupCommand) Description() string           { return "Configure terminal settings" }
func (c *TerminalSetupCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *TerminalSetupCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *TerminalSetupCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "terminal-setup"}, nil
}

func init() {
	defaultRegistry.Register(
		&AddDirCommand{},
		&HooksCommand{},
		&FeedbackCommand{},
		&StatsCommand{},
		&AdvisorCommand{},
		&TagCommand{},
		&UsageCommand{},
		&DesktopCommand{},
		&PrivacySettingsCommand{},
		&UpgradeCommand{},
		&ReloadPluginsCommand{},
		&BridgeCommand{},
		&BridgeKickCommand{},
		&BtwCommand{},
		&PassesCommand{},
		&ReleaseNotesCommand{},
		&TerminalSetupCommand{},
	)
}
