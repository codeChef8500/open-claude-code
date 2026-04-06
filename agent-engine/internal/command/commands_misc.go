package command

import (
	"context"
	"fmt"
	"strings"
)

// AddDirCommand shell removed — replaced by DeepAddDirCommand in commands_deep_p1.go.

// ─── /hooks ──────────────────────────────────────────────────────────────────

// HooksCommand shell removed — replaced by DeepHooksCommand in commands_deep_p1.go.

// ─── /feedback ───────────────────────────────────────────────────────────────

// FeedbackCommand shell removed — replaced by DeepFeedbackCommand in commands_deep_p1.go.

// ─── /stats ──────────────────────────────────────────────────────────────────

// StatsCommand shell removed — replaced by DeepStatsCommand in commands_deep_p1.go.

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

// DesktopCommand shell removed — replaced by DeepDesktopCommand in commands_deep_p2.go.
// PrivacySettingsCommand shell removed — replaced by DeepPrivacySettingsCommand in commands_deep_p2.go.
// UpgradeCommand shell removed — replaced by DeepUpgradeCommand in commands_deep_p2.go.

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

// BtwCommand shell removed — replaced by DeepBtwCommand in commands_deep_p2.go.
// ReleaseNotesCommand shell removed — replaced by DeepReleaseNotesCommand in commands_deep_p2.go.
// TerminalSetupCommand shell removed — replaced by DeepTerminalSetupCommand in commands_deep_p2.go.

func init() {
	defaultRegistry.Register(
		&AdvisorCommand{},
		&TagCommand{},
		&ReloadPluginsCommand{},
		&BridgeKickCommand{},
	)
}
