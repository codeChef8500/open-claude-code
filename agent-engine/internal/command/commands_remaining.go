package command

import "context"

// This file implements the remaining commands from claude-code-main that were
// not yet ported. Many are internal-only, feature-gated, or platform-specific.

// MobileCommand shell removed — replaced by DeepMobileCommand in commands_deep_p3.go.
// ChromeCommand shell removed — replaced by DeepChromeCommand in commands_deep_p3.go.
// IDECommand shell removed — replaced by DeepIDECommand in commands_deep_p3.go.

// SandboxToggleCommand shell removed — replaced by DeepSandboxToggleCommand in commands_deep_p2.go.
// RateLimitOptionsCommand shell removed — replaced by DeepRateLimitOptionsCommand in commands_deep_p2.go.

// InstallGitHubAppCommand shell removed — replaced by DeepInstallGitHubAppCommand in commands_deep_p3.go.
// InstallSlackAppCommand shell removed — replaced by DeepInstallSlackAppCommand in commands_deep_p3.go.
// RemoteEnvCommand shell removed — replaced by DeepRemoteEnvCommand in commands_deep_p3.go.
// RemoteSetupCommand shell removed — replaced by DeepRemoteSetupCommand in commands_deep_p3.go.

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

// ThinkbackCommand shell removed — replaced by DeepThinkbackCommand in commands_deep_p3.go.
// ThinkbackPlayCommand shell removed — replaced by DeepThinkbackPlayCommand in commands_deep_p3.go.

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
		&FilesCommand{},
		&InsightsCommand{},
		&InitVerifiersCommand{},
		&HeapdumpCommand{},
	)
}
