package command

import "context"

// This file implements the remaining commands from claude-code-main that were
// not yet ported. Many are internal-only, feature-gated, or platform-specific.

// ─── /mobile ─────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/mobile/index.ts (local-jsx).

type MobileCommand struct{ BaseCommand }

func (c *MobileCommand) Name() string                  { return "mobile" }
func (c *MobileCommand) Description() string           { return "Connect mobile device" }
func (c *MobileCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *MobileCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *MobileCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "mobile"}, nil
}

// ─── /chrome ─────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/chrome/index.ts (local-jsx).

type ChromeCommand struct{ BaseCommand }

func (c *ChromeCommand) Name() string                  { return "chrome" }
func (c *ChromeCommand) Description() string           { return "Manage Chrome browser automation" }
func (c *ChromeCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ChromeCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ChromeCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "chrome"}, nil
}

// ─── /ide ────────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/ide/index.ts (local-jsx).

type IDECommand struct{ BaseCommand }

func (c *IDECommand) Name() string                  { return "ide" }
func (c *IDECommand) Description() string           { return "Manage IDE integration settings" }
func (c *IDECommand) Type() CommandType             { return CommandTypeInteractive }
func (c *IDECommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *IDECommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "ide"}, nil
}

// ─── /sandbox-toggle ─────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/sandbox-toggle/index.ts (local-jsx).

type SandboxToggleCommand struct{ BaseCommand }

func (c *SandboxToggleCommand) Name() string                  { return "sandbox-toggle" }
func (c *SandboxToggleCommand) Aliases() []string             { return []string{"sandbox"} }
func (c *SandboxToggleCommand) Description() string           { return "Toggle sandbox mode" }
func (c *SandboxToggleCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *SandboxToggleCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *SandboxToggleCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "sandbox-toggle"}, nil
}

// ─── /rate-limit-options ─────────────────────────────────────────────────────
// Aligned with claude-code-main commands/rate-limit-options/index.ts (local-jsx).

type RateLimitOptionsCommand struct{ BaseCommand }

func (c *RateLimitOptionsCommand) Name() string        { return "rate-limit-options" }
func (c *RateLimitOptionsCommand) Description() string { return "View rate limit options" }
func (c *RateLimitOptionsCommand) Availability() []CommandAvailability {
	return []CommandAvailability{AvailabilityClaudeAI}
}
func (c *RateLimitOptionsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *RateLimitOptionsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *RateLimitOptionsCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "rate-limit-options"}, nil
}

// ─── /install-github-app ─────────────────────────────────────────────────────
// Aligned with claude-code-main commands/install-github-app/index.ts (local-jsx).

type InstallGitHubAppCommand struct{ BaseCommand }

func (c *InstallGitHubAppCommand) Name() string                  { return "install-github-app" }
func (c *InstallGitHubAppCommand) Description() string           { return "Install the GitHub App integration" }
func (c *InstallGitHubAppCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *InstallGitHubAppCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *InstallGitHubAppCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "install-github-app"}, nil
}

// ─── /install-slack-app ──────────────────────────────────────────────────────
// Aligned with claude-code-main commands/install-slack-app/index.ts (local-jsx).

type InstallSlackAppCommand struct{ BaseCommand }

func (c *InstallSlackAppCommand) Name() string                  { return "install-slack-app" }
func (c *InstallSlackAppCommand) Description() string           { return "Install the Slack App integration" }
func (c *InstallSlackAppCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *InstallSlackAppCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *InstallSlackAppCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "install-slack-app"}, nil
}

// ─── /remote-env ─────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/remote-env/index.ts (local-jsx).

type RemoteEnvCommand struct{ BaseCommand }

func (c *RemoteEnvCommand) Name() string                  { return "remote-env" }
func (c *RemoteEnvCommand) Description() string           { return "Configure remote environment settings" }
func (c *RemoteEnvCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *RemoteEnvCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *RemoteEnvCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "remote-env"}, nil
}

// ─── /remote-setup ───────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/remote-setup/index.ts (local-jsx).

type RemoteSetupCommand struct{ BaseCommand }

func (c *RemoteSetupCommand) Name() string                  { return "remote-setup" }
func (c *RemoteSetupCommand) Description() string           { return "Configure remote setup" }
func (c *RemoteSetupCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *RemoteSetupCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *RemoteSetupCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "remote-setup"}, nil
}

// ─── /files ──────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/files/index.ts (local, ant-only).

type FilesCommand struct{ BaseCommand }

func (c *FilesCommand) Name() string                  { return "files" }
func (c *FilesCommand) Description() string           { return "List all files currently in context" }
func (c *FilesCommand) IsHidden() bool                { return true }
func (c *FilesCommand) Type() CommandType             { return CommandTypeLocal }
func (c *FilesCommand) IsEnabled(_ *ExecContext) bool { return false } // ant-only
func (c *FilesCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "Files in context: (not available)", nil
}

// ─── /thinkback ──────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/thinkback/index.ts (local-jsx).

type ThinkbackCommand struct{ BaseCommand }

func (c *ThinkbackCommand) Name() string                  { return "thinkback" }
func (c *ThinkbackCommand) Description() string           { return "View and replay thinking traces" }
func (c *ThinkbackCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ThinkbackCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ThinkbackCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "thinkback"}, nil
}

// ─── /thinkback-play ─────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/thinkback-play/index.ts (local-jsx).

type ThinkbackPlayCommand struct{ BaseCommand }

func (c *ThinkbackPlayCommand) Name() string                  { return "thinkback-play" }
func (c *ThinkbackPlayCommand) Description() string           { return "Play back thinking traces" }
func (c *ThinkbackPlayCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ThinkbackPlayCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ThinkbackPlayCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "thinkback-play"}, nil
}

// ─── /insights ───────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/insights.ts (prompt).

type InsightsCommand struct{ BasePromptCommand }

func (c *InsightsCommand) Name() string                  { return "insights" }
func (c *InsightsCommand) Description() string           { return "Generate insights about the codebase" }
func (c *InsightsCommand) Type() CommandType             { return CommandTypePrompt }
func (c *InsightsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *InsightsCommand) PromptContent(_ []string, _ *ExecContext) (string, error) {
	return `## Your Task

Analyze this codebase and provide insights about:

1. **Architecture**: Overall structure, patterns, and design decisions
2. **Code Quality**: Potential issues, technical debt, areas for improvement
3. **Dependencies**: Key dependencies and their roles
4. **Testing**: Test coverage and quality observations
5. **Performance**: Potential bottlenecks or optimization opportunities

Be specific, reference actual files and patterns you find.`, nil
}

// ─── /init-verifiers ─────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/init-verifiers.ts (prompt).

type InitVerifiersCommand struct{ BasePromptCommand }

func (c *InitVerifiersCommand) Name() string                  { return "init-verifiers" }
func (c *InitVerifiersCommand) Description() string           { return "Initialize verifier configurations" }
func (c *InitVerifiersCommand) Type() CommandType             { return CommandTypePrompt }
func (c *InitVerifiersCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *InitVerifiersCommand) PromptContent(_ []string, _ *ExecContext) (string, error) {
	return `## Initialize Verifiers

Set up verification rules for this project:

1. Analyze the project's test framework and build system
2. Create or update .claude/verifiers.json with appropriate checks
3. Include lint, type-check, test, and build verification commands
4. Ensure verifiers match the project's actual toolchain`, nil
}

// ─── /heapdump ───────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/heapdump/index.ts (local-jsx, hidden).

type HeapdumpCommand struct{ BaseCommand }

func (c *HeapdumpCommand) Name() string                  { return "heapdump" }
func (c *HeapdumpCommand) Description() string           { return "Create a heap dump for debugging" }
func (c *HeapdumpCommand) IsHidden() bool                { return true }
func (c *HeapdumpCommand) Type() CommandType             { return CommandTypeLocal }
func (c *HeapdumpCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *HeapdumpCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "Heap dump not available in Go runtime (use pprof instead).", nil
}

func init() {
	defaultRegistry.Register(
		&MobileCommand{},
		&ChromeCommand{},
		&IDECommand{},
		&SandboxToggleCommand{},
		&RateLimitOptionsCommand{},
		&InstallGitHubAppCommand{},
		&InstallSlackAppCommand{},
		&RemoteEnvCommand{},
		&RemoteSetupCommand{},
		&FilesCommand{},
		&ThinkbackCommand{},
		&ThinkbackPlayCommand{},
		&InsightsCommand{},
		&InitVerifiersCommand{},
		&HeapdumpCommand{},
	)
}
