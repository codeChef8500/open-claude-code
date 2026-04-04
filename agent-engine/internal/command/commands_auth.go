package command

import "context"

// ─── /login ──────────────────────────────────────────────────────────────────

// LoginCommand signs in with an Anthropic account.
// Aligned with claude-code-main commands/login/index.ts (local-jsx).
type LoginCommand struct{ BaseCommand }

func (c *LoginCommand) Name() string                  { return "login" }
func (c *LoginCommand) Description() string           { return "Sign in with your Anthropic account" }
func (c *LoginCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *LoginCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *LoginCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "login"}, nil
}

// ─── /logout ─────────────────────────────────────────────────────────────────

// LogoutCommand signs out from an Anthropic account.
// Aligned with claude-code-main commands/logout/index.ts (local-jsx).
type LogoutCommand struct{ BaseCommand }

func (c *LogoutCommand) Name() string                  { return "logout" }
func (c *LogoutCommand) Description() string           { return "Sign out from your Anthropic account" }
func (c *LogoutCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *LogoutCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *LogoutCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "logout"}, nil
}

func init() {
	defaultRegistry.Register(
		&LoginCommand{},
		&LogoutCommand{},
	)
}
