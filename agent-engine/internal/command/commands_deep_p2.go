package command

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// P2 Deep Implementations: Account/platform commands.
// /upgrade, /privacy-settings, /desktop, /btw, /terminal-setup, /release-notes,
// /sandbox-toggle, /rate-limit-options.
// ──────────────────────────────────────────────────────────────────────────────

// ─── /upgrade deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/upgrade/upgrade.tsx.
// Checks subscription plan and opens upgrade page.

// UpgradeViewData is the structured data for the upgrade TUI component.
type UpgradeViewData struct {
	CurrentPlan  string `json:"current_plan,omitempty"`
	IsMaxPlan    bool   `json:"is_max_plan"`
	UpgradeURL   string `json:"upgrade_url"`
	NeedRelogin  bool   `json:"need_relogin"`
	Error        string `json:"error,omitempty"`
}

// DeepUpgradeCommand replaces the basic UpgradeCommand.
type DeepUpgradeCommand struct{ BaseCommand }

func (c *DeepUpgradeCommand) Name() string                  { return "upgrade" }
func (c *DeepUpgradeCommand) Aliases() []string             { return []string{"update"} }
func (c *DeepUpgradeCommand) Description() string           { return "Upgrade to the latest version" }
func (c *DeepUpgradeCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepUpgradeCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepUpgradeCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &UpgradeViewData{
		UpgradeURL: "https://claude.ai/upgrade/max",
	}

	if ectx != nil && ectx.Services != nil && ectx.Services.Auth != nil {
		if ectx.Services.Auth.IsClaudeAISubscriber() {
			data.CurrentPlan = "subscriber"
		}
	}

	return &InteractiveResult{
		Component: "upgrade",
		Data:      data,
	}, nil
}

// ─── /privacy-settings deep implementation ───────────────────────────────────
// Aligned with claude-code-main commands/privacy-settings/privacy-settings.tsx.
// Shows privacy settings dialog or Grove enrollment.

// PrivacySettingsViewData is the structured data for the privacy settings TUI.
type PrivacySettingsViewData struct {
	IsQualified     bool   `json:"is_qualified"`
	TermsAccepted   bool   `json:"terms_accepted"`
	PrivacyEnabled  bool   `json:"privacy_enabled"`
	Error           string `json:"error,omitempty"`
}

// DeepPrivacySettingsCommand replaces the basic PrivacySettingsCommand.
type DeepPrivacySettingsCommand struct{ BaseCommand }

func (c *DeepPrivacySettingsCommand) Name() string                  { return "privacy-settings" }
func (c *DeepPrivacySettingsCommand) Aliases() []string             { return []string{"privacy"} }
func (c *DeepPrivacySettingsCommand) Description() string           { return "Configure privacy settings" }
func (c *DeepPrivacySettingsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepPrivacySettingsCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepPrivacySettingsCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &PrivacySettingsViewData{}

	if ectx != nil && ectx.Services != nil && ectx.Services.Config != nil {
		cfg := ectx.Services.Config
		if v, ok := cfg.Get("privacyMode"); ok {
			if b, ok := v.(bool); ok {
				data.PrivacyEnabled = b
			}
		}
	}

	return &InteractiveResult{
		Component: "privacy-settings",
		Data:      data,
	}, nil
}

// ─── /desktop deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/desktop/desktop.tsx.
// Facilitates handoff to desktop application.

// DesktopViewData is the structured data for the desktop TUI component.
type DesktopViewData struct {
	Platform    string `json:"platform"`
	SessionID   string `json:"session_id,omitempty"`
	HandoffURL  string `json:"handoff_url,omitempty"`
}

// DeepDesktopCommand replaces the basic DesktopCommand.
type DeepDesktopCommand struct{ BaseCommand }

func (c *DeepDesktopCommand) Name() string                  { return "desktop" }
func (c *DeepDesktopCommand) Description() string           { return "Manage desktop app settings" }
func (c *DeepDesktopCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepDesktopCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepDesktopCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &DesktopViewData{
		Platform: runtime.GOOS,
	}
	if ectx != nil {
		data.SessionID = ectx.SessionID
	}

	return &InteractiveResult{
		Component: "desktop",
		Data:      data,
	}, nil
}

// ─── /btw deep implementation ────────────────────────────────────────────────
// Aligned with claude-code-main commands/btw/btw.tsx.
// Sends a side question fork message.

// BtwViewData is the structured data for the btw TUI component.
type BtwViewData struct {
	Message   string `json:"message,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	TurnCount int    `json:"turn_count,omitempty"`
}

// DeepBtwCommand replaces the basic BtwCommand.
type DeepBtwCommand struct{ BaseCommand }

func (c *DeepBtwCommand) Name() string                  { return "btw" }
func (c *DeepBtwCommand) Description() string           { return "Send a side message to the model" }
func (c *DeepBtwCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepBtwCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepBtwCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &BtwViewData{}
	if len(args) > 0 {
		data.Message = strings.Join(args, " ")
	}
	if ectx != nil {
		data.SessionID = ectx.SessionID
		data.TurnCount = ectx.TurnCount
	}

	return &InteractiveResult{
		Component: "btw",
		Data:      data,
	}, nil
}

// ─── /terminal-setup deep implementation ─────────────────────────────────────
// Aligned with claude-code-main commands/terminalSetup/terminalSetup.tsx.
// Provides terminal configuration guidance for Shift+Enter support.

// TerminalSetupViewData is the structured data for the terminal-setup TUI.
type TerminalSetupViewData struct {
	Platform       string            `json:"platform"`
	Terminal       string            `json:"terminal,omitempty"`
	SupportsNative bool              `json:"supports_native"`
	Instructions   map[string]string `json:"instructions,omitempty"`
}

// DeepTerminalSetupCommand replaces the basic TerminalSetupCommand.
type DeepTerminalSetupCommand struct{ BaseCommand }

func (c *DeepTerminalSetupCommand) Name() string                  { return "terminal-setup" }
func (c *DeepTerminalSetupCommand) Aliases() []string             { return []string{"terminalsetup"} }
func (c *DeepTerminalSetupCommand) Description() string           { return "Configure terminal settings" }
func (c *DeepTerminalSetupCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepTerminalSetupCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepTerminalSetupCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &TerminalSetupViewData{
		Platform: runtime.GOOS,
		Instructions: map[string]string{
			"vscode":    "Shift+Enter is supported natively in VSCode terminal.",
			"cursor":    "Shift+Enter is supported natively in Cursor terminal.",
			"windsurf":  "Shift+Enter is supported natively in Windsurf terminal.",
			"zed":       "Shift+Enter is supported natively in Zed terminal.",
			"alacritty": "Add key binding in alacritty.toml: [key_bindings] { key = \"Return\", mods = \"Shift\", chars = \"\\n\" }",
			"iterm2":    "Shift+Enter is natively supported in iTerm2.",
			"wezterm":   "Shift+Enter is natively supported in WezTerm.",
			"ghostty":   "Shift+Enter is natively supported in Ghostty.",
			"kitty":     "Shift+Enter is natively supported in Kitty.",
			"warp":      "Shift+Enter is natively supported in Warp.",
		},
	}

	return &InteractiveResult{
		Component: "terminal-setup",
		Data:      data,
	}, nil
}

// ─── /release-notes deep implementation ──────────────────────────────────────
// Aligned with claude-code-main commands/release-notes/release-notes.tsx.
// Shows version changelog.

// ReleaseNotesViewData is the structured data for the release-notes TUI.
type ReleaseNotesViewData struct {
	CurrentVersion string             `json:"current_version"`
	Entries        []ReleaseNoteEntry `json:"entries,omitempty"`
}

// ReleaseNoteEntry describes a single release note.
type ReleaseNoteEntry struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	Summary string `json:"summary"`
}

// DeepReleaseNotesCommand replaces the basic ReleaseNotesCommand.
type DeepReleaseNotesCommand struct{ BaseCommand }

func (c *DeepReleaseNotesCommand) Name() string                  { return "release-notes" }
func (c *DeepReleaseNotesCommand) Aliases() []string             { return []string{"changelog"} }
func (c *DeepReleaseNotesCommand) Description() string           { return "Show release notes" }
func (c *DeepReleaseNotesCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepReleaseNotesCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepReleaseNotesCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &ReleaseNotesViewData{
		CurrentVersion: "0.1.0",
		Entries: []ReleaseNoteEntry{
			{Version: "0.1.0", Date: "2025-01-01", Summary: "Initial release of agent-engine with full command module alignment."},
		},
	}

	return &InteractiveResult{
		Component: "release-notes",
		Data:      data,
	}, nil
}

// ─── /sandbox-toggle deep implementation ─────────────────────────────────────
// Aligned with claude-code-main commands/sandbox-toggle/sandbox-toggle.tsx.
// Toggles sandbox mode with optional exclude patterns.

// SandboxToggleViewData is the structured data for the sandbox-toggle TUI.
type SandboxToggleViewData struct {
	Enabled        bool     `json:"enabled"`
	Supported      bool     `json:"supported"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`
	Subcommand     string   `json:"subcommand,omitempty"`
	PolicyLocked   bool     `json:"policy_locked"`
	Error          string   `json:"error,omitempty"`
}

// DeepSandboxToggleCommand replaces the basic SandboxToggleCommand.
type DeepSandboxToggleCommand struct{ BaseCommand }

func (c *DeepSandboxToggleCommand) Name() string                  { return "sandbox-toggle" }
func (c *DeepSandboxToggleCommand) Aliases() []string             { return []string{"sandbox"} }
func (c *DeepSandboxToggleCommand) Description() string           { return "Toggle sandbox mode" }
func (c *DeepSandboxToggleCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepSandboxToggleCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepSandboxToggleCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &SandboxToggleViewData{
		Supported: runtime.GOOS == "darwin" || runtime.GOOS == "linux",
	}

	if len(args) > 0 {
		data.Subcommand = args[0]
	}

	// Read current sandbox state from config.
	if ectx != nil && ectx.Services != nil && ectx.Services.Config != nil {
		cfg := ectx.Services.Config
		if v, ok := cfg.Get("sandboxMode"); ok {
			if b, ok := v.(bool); ok {
				data.Enabled = b
			}
		}
		if v, ok := cfg.Get("sandboxExclude"); ok {
			if patterns, ok := v.([]interface{}); ok {
				for _, p := range patterns {
					if s, ok := p.(string); ok {
						data.ExcludePatterns = append(data.ExcludePatterns, s)
					}
				}
			}
		}
	}

	return &InteractiveResult{
		Component: "sandbox-toggle",
		Data:      data,
	}, nil
}

// ─── /rate-limit-options deep implementation ─────────────────────────────────
// Aligned with claude-code-main commands/rate-limit-options/rate-limit-options.tsx.
// Shows rate limit options and subscription plans.

// RateLimitOptionsViewData is the structured data for the rate-limit-options TUI.
type RateLimitOptionsViewData struct {
	CurrentPlan string                `json:"current_plan,omitempty"`
	Options     []RateLimitOption     `json:"options,omitempty"`
	UpgradeURL  string                `json:"upgrade_url,omitempty"`
}

// RateLimitOption describes a subscription tier option.
type RateLimitOption struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Limit       string `json:"limit"`
	IsCurrent   bool   `json:"is_current"`
}

// DeepRateLimitOptionsCommand replaces the basic RateLimitOptionsCommand.
type DeepRateLimitOptionsCommand struct{ BaseCommand }

func (c *DeepRateLimitOptionsCommand) Name() string        { return "rate-limit-options" }
func (c *DeepRateLimitOptionsCommand) Description() string { return "View rate limit options" }
func (c *DeepRateLimitOptionsCommand) Availability() []CommandAvailability {
	return []CommandAvailability{AvailabilityClaudeAI}
}
func (c *DeepRateLimitOptionsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepRateLimitOptionsCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepRateLimitOptionsCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &RateLimitOptionsViewData{
		UpgradeURL: "https://claude.ai/upgrade",
		Options: []RateLimitOption{
			{Name: "Free", Description: "Basic access with standard limits", Limit: "Limited"},
			{Name: "Pro", Description: "Higher limits for professional use", Limit: "5x Free"},
			{Name: "Max", Description: "Maximum throughput for heavy use", Limit: "20x Free"},
		},
	}

	if ectx != nil && ectx.Services != nil && ectx.Services.Auth != nil {
		if ectx.Services.Auth.IsClaudeAISubscriber() {
			data.CurrentPlan = "pro"
			if len(data.Options) > 1 {
				data.Options[1].IsCurrent = true
			}
		} else {
			data.CurrentPlan = "free"
			if len(data.Options) > 0 {
				data.Options[0].IsCurrent = true
			}
		}
	}

	return &InteractiveResult{
		Component: "rate-limit-options",
		Data:      data,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Unused import guard
// ──────────────────────────────────────────────────────────────────────────────

var _ = fmt.Sprintf // ensure fmt is used

// ──────────────────────────────────────────────────────────────────────────────
// Register P2 deep commands, replacing stubs.
// ──────────────────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepUpgradeCommand{},
		&DeepPrivacySettingsCommand{},
		&DeepDesktopCommand{},
		&DeepBtwCommand{},
		&DeepTerminalSetupCommand{},
		&DeepReleaseNotesCommand{},
		&DeepSandboxToggleCommand{},
		&DeepRateLimitOptionsCommand{},
	)
}
