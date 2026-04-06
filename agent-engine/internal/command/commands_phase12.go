package command

import (
	"context"
	"fmt"
	"runtime"
)

// Stubs for config, mcp, init, review, commit, doctor, bug-report removed —
// replaced by deep implementations in config_impl.go, mcp_impl.go,
// commands_prompt_advanced.go, doctor_impl.go.

// ─── /version ────────────────────────────────────────────────────────────────

// VersionCommand shows the agent engine version.
type VersionCommand struct{ BaseCommand }

func (c *VersionCommand) Name() string                  { return "version" }
func (c *VersionCommand) Aliases() []string             { return []string{"v"} }
func (c *VersionCommand) Description() string           { return "Show agent engine version information." }
func (c *VersionCommand) Type() CommandType             { return CommandTypeLocal }
func (c *VersionCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *VersionCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return fmt.Sprintf("Agent Engine v0.1.0\nGo %s\nOS/Arch: %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH), nil
}

// ─── /quit ───────────────────────────────────────────────────────────────────

// QuitCommand exits the session.
type QuitCommand struct{ BaseCommand }

func (c *QuitCommand) Name() string                  { return "quit" }
func (c *QuitCommand) Aliases() []string             { return []string{"q", "exit"} }
func (c *QuitCommand) Description() string           { return "Exit the current session" }
func (c *QuitCommand) IsImmediate() bool             { return true }
func (c *QuitCommand) Type() CommandType             { return CommandTypeLocal }
func (c *QuitCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *QuitCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "__quit__", nil
}

// ─── Register Phase 12 commands ──────────────────────────────────────────────

func init() {
	defaultRegistry.Register(
		&VersionCommand{},
		&QuitCommand{},
	)
}
