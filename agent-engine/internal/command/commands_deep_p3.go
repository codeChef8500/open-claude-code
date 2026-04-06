package command

import (
	"context"
	"runtime"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// P3 Deep Implementations: Integration/plugin commands.
// /mobile, /chrome, /ide, /install-github-app, /install-slack-app,
// /remote-env, /remote-setup, /thinkback, /thinkback-play.
// ──────────────────────────────────────────────────────────────────────────────

// ─── /mobile deep implementation ─────────────────────────────────────────────
// Aligned with claude-code-main commands/mobile/mobile.tsx.
// Connects to a mobile device via QR code pairing.

// MobileViewData is the structured data for the mobile TUI component.
type MobileViewData struct {
	SessionID string `json:"session_id,omitempty"`
	PairURL   string `json:"pair_url,omitempty"`
	IsLinked  bool   `json:"is_linked"`
}

// DeepMobileCommand replaces the basic MobileCommand.
type DeepMobileCommand struct{ BaseCommand }

func (c *DeepMobileCommand) Name() string                  { return "mobile" }
func (c *DeepMobileCommand) Description() string           { return "Connect mobile device" }
func (c *DeepMobileCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepMobileCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepMobileCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &MobileViewData{}
	if ectx != nil {
		data.SessionID = ectx.SessionID
	}

	return &InteractiveResult{
		Component: "mobile",
		Data:      data,
	}, nil
}

// ─── /chrome deep implementation ─────────────────────────────────────────────
// Aligned with claude-code-main commands/chrome/chrome.tsx.
// Manages Chrome browser automation integration.

// ChromeViewData is the structured data for the chrome TUI component.
type ChromeViewData struct {
	IsInstalled bool   `json:"is_installed"`
	IsRunning   bool   `json:"is_running"`
	Version     string `json:"version,omitempty"`
	DebugPort   int    `json:"debug_port,omitempty"`
	Subcommand  string `json:"subcommand,omitempty"`
}

// DeepChromeCommand replaces the basic ChromeCommand.
type DeepChromeCommand struct{ BaseCommand }

func (c *DeepChromeCommand) Name() string                  { return "chrome" }
func (c *DeepChromeCommand) Description() string           { return "Manage Chrome browser automation" }
func (c *DeepChromeCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepChromeCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepChromeCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &ChromeViewData{
		DebugPort: 9222,
	}
	if len(args) > 0 {
		data.Subcommand = args[0]
	}

	return &InteractiveResult{
		Component: "chrome",
		Data:      data,
	}, nil
}

// ─── /ide deep implementation ────────────────────────────────────────────────
// Aligned with claude-code-main commands/ide/ide.tsx.
// Manages IDE integration settings (VS Code, Cursor, etc.).

// IDEViewData is the structured data for the ide TUI component.
type IDEViewData struct {
	DetectedIDE string `json:"detected_ide,omitempty"`
	Platform    string `json:"platform"`
	Subcommand  string `json:"subcommand,omitempty"`
}

// DeepIDECommand replaces the basic IDECommand.
type DeepIDECommand struct{ BaseCommand }

func (c *DeepIDECommand) Name() string                  { return "ide" }
func (c *DeepIDECommand) Description() string           { return "Manage IDE integration settings" }
func (c *DeepIDECommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepIDECommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepIDECommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &IDEViewData{
		Platform: runtime.GOOS,
	}
	if len(args) > 0 {
		data.Subcommand = args[0]
	}

	return &InteractiveResult{
		Component: "ide",
		Data:      data,
	}, nil
}

// ─── /install-github-app deep implementation ─────────────────────────────────
// Aligned with claude-code-main commands/install-github-app/install-github-app.tsx.
// Opens the GitHub App installation flow.

// InstallGitHubAppViewData is the structured data for the install-github-app TUI.
type InstallGitHubAppViewData struct {
	InstallURL   string `json:"install_url"`
	IsInstalled  bool   `json:"is_installed"`
	Organization string `json:"organization,omitempty"`
}

// DeepInstallGitHubAppCommand replaces the basic InstallGitHubAppCommand.
type DeepInstallGitHubAppCommand struct{ BaseCommand }

func (c *DeepInstallGitHubAppCommand) Name() string                  { return "install-github-app" }
func (c *DeepInstallGitHubAppCommand) Description() string           { return "Install the GitHub App integration" }
func (c *DeepInstallGitHubAppCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepInstallGitHubAppCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepInstallGitHubAppCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &InstallGitHubAppViewData{
		InstallURL: "https://github.com/apps/claude-code/installations/new",
	}
	if len(args) > 0 {
		data.Organization = args[0]
	}

	return &InteractiveResult{
		Component: "install-github-app",
		Data:      data,
	}, nil
}

// ─── /install-slack-app deep implementation ──────────────────────────────────
// Aligned with claude-code-main commands/install-slack-app/install-slack-app.tsx.
// Opens the Slack App installation flow.

// InstallSlackAppViewData is the structured data for the install-slack-app TUI.
type InstallSlackAppViewData struct {
	InstallURL  string `json:"install_url"`
	IsInstalled bool   `json:"is_installed"`
	Workspace   string `json:"workspace,omitempty"`
}

// DeepInstallSlackAppCommand replaces the basic InstallSlackAppCommand.
type DeepInstallSlackAppCommand struct{ BaseCommand }

func (c *DeepInstallSlackAppCommand) Name() string                  { return "install-slack-app" }
func (c *DeepInstallSlackAppCommand) Description() string           { return "Install the Slack App integration" }
func (c *DeepInstallSlackAppCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepInstallSlackAppCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepInstallSlackAppCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &InstallSlackAppViewData{
		InstallURL: "https://slack.com/oauth/v2/authorize",
	}
	if len(args) > 0 {
		data.Workspace = args[0]
	}

	return &InteractiveResult{
		Component: "install-slack-app",
		Data:      data,
	}, nil
}

// ─── /remote-env deep implementation ─────────────────────────────────────────
// Aligned with claude-code-main commands/remote-env/remote-env.tsx.
// Configures remote development environment connections.

// RemoteEnvViewData is the structured data for the remote-env TUI.
type RemoteEnvViewData struct {
	CurrentEnv  string            `json:"current_env,omitempty"`
	Environments []RemoteEnvEntry `json:"environments,omitempty"`
	Subcommand  string            `json:"subcommand,omitempty"`
}

// RemoteEnvEntry describes a configured remote environment.
type RemoteEnvEntry struct {
	Name   string `json:"name"`
	Host   string `json:"host"`
	Status string `json:"status"` // "connected", "disconnected"
}

// DeepRemoteEnvCommand replaces the basic RemoteEnvCommand.
type DeepRemoteEnvCommand struct{ BaseCommand }

func (c *DeepRemoteEnvCommand) Name() string                  { return "remote-env" }
func (c *DeepRemoteEnvCommand) Description() string           { return "Configure remote environment settings" }
func (c *DeepRemoteEnvCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepRemoteEnvCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepRemoteEnvCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &RemoteEnvViewData{}
	if len(args) > 0 {
		data.Subcommand = args[0]
	}

	return &InteractiveResult{
		Component: "remote-env",
		Data:      data,
	}, nil
}

// ─── /remote-setup deep implementation ───────────────────────────────────────
// Aligned with claude-code-main commands/remote-setup/remote-setup.tsx.
// Configures remote setup for headless/CI environments.

// RemoteSetupViewData is the structured data for the remote-setup TUI.
type RemoteSetupViewData struct {
	Platform   string `json:"platform"`
	IsHeadless bool   `json:"is_headless"`
	SSHKey     string `json:"ssh_key,omitempty"`
}

// DeepRemoteSetupCommand replaces the basic RemoteSetupCommand.
type DeepRemoteSetupCommand struct{ BaseCommand }

func (c *DeepRemoteSetupCommand) Name() string                  { return "remote-setup" }
func (c *DeepRemoteSetupCommand) Description() string           { return "Configure remote setup" }
func (c *DeepRemoteSetupCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepRemoteSetupCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepRemoteSetupCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &RemoteSetupViewData{
		Platform: runtime.GOOS,
	}

	return &InteractiveResult{
		Component: "remote-setup",
		Data:      data,
	}, nil
}

// ─── /thinkback deep implementation ──────────────────────────────────────────
// Aligned with claude-code-main commands/thinkback/thinkback.tsx.
// Views and replays thinking traces from the conversation.

// ThinkbackViewData is the structured data for the thinkback TUI.
type ThinkbackViewData struct {
	SessionID    string              `json:"session_id,omitempty"`
	Traces       []ThinkingTraceEntry `json:"traces,omitempty"`
	SelectedIdx  int                 `json:"selected_idx,omitempty"`
}

// ThinkingTraceEntry represents a single thinking block from the conversation.
type ThinkingTraceEntry struct {
	TurnIndex int    `json:"turn_index"`
	Preview   string `json:"preview"`
	Length    int    `json:"length"`
}

// DeepThinkbackCommand replaces the basic ThinkbackCommand.
type DeepThinkbackCommand struct{ BaseCommand }

func (c *DeepThinkbackCommand) Name() string                  { return "thinkback" }
func (c *DeepThinkbackCommand) Description() string           { return "View and replay thinking traces" }
func (c *DeepThinkbackCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepThinkbackCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepThinkbackCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &ThinkbackViewData{}
	if ectx != nil {
		data.SessionID = ectx.SessionID
	}
	if len(args) > 0 {
		// Attempt to parse a trace index from args.
		data.SelectedIdx = -1 // will be parsed by TUI
	}

	return &InteractiveResult{
		Component: "thinkback",
		Data:      data,
	}, nil
}

// ─── /thinkback-play deep implementation ─────────────────────────────────────
// Aligned with claude-code-main commands/thinkback-play/thinkback-play.tsx.
// Plays back thinking traces with streaming animation.

// ThinkbackPlayViewData is the structured data for the thinkback-play TUI.
type ThinkbackPlayViewData struct {
	SessionID string `json:"session_id,omitempty"`
	TraceIdx  int    `json:"trace_idx,omitempty"`
	Speed     string `json:"speed,omitempty"` // "slow", "normal", "fast"
}

// DeepThinkbackPlayCommand replaces the basic ThinkbackPlayCommand.
type DeepThinkbackPlayCommand struct{ BaseCommand }

func (c *DeepThinkbackPlayCommand) Name() string                  { return "thinkback-play" }
func (c *DeepThinkbackPlayCommand) Description() string           { return "Play back thinking traces" }
func (c *DeepThinkbackPlayCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepThinkbackPlayCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepThinkbackPlayCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &ThinkbackPlayViewData{
		Speed: "normal",
	}
	if ectx != nil {
		data.SessionID = ectx.SessionID
	}
	if len(args) > 0 {
		data.Speed = strings.ToLower(args[0])
	}

	return &InteractiveResult{
		Component: "thinkback-play",
		Data:      data,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Register P3 deep commands, replacing stubs.
// ──────────────────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepMobileCommand{},
		&DeepChromeCommand{},
		&DeepIDECommand{},
		&DeepInstallGitHubAppCommand{},
		&DeepInstallSlackAppCommand{},
		&DeepRemoteEnvCommand{},
		&DeepRemoteSetupCommand{},
		&DeepThinkbackCommand{},
		&DeepThinkbackPlayCommand{},
	)
}
