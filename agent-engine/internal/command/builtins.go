package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /help ────────────────────────────────────────────────────────────────────

type HelpCommand struct{}

func (c *HelpCommand) Name() string                         { return "help" }
func (c *HelpCommand) Description() string                  { return "Show available slash commands." }
func (c *HelpCommand) Type() CommandType                    { return CommandTypeLocal }
func (c *HelpCommand) IsEnabled(_ *ExecContext) bool        { return true }
func (c *HelpCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "", nil
	}
	lines := []string{"Available commands:"}
	for _, cmd := range defaultRegistry.Enabled(ectx) {
		lines = append(lines, fmt.Sprintf("  /%s — %s", cmd.Name(), cmd.Description()))
	}
	return strings.Join(lines, "\n"), nil
}

// ─── /clear ───────────────────────────────────────────────────────────────────

type ClearCommand struct{}

func (c *ClearCommand) Name() string                         { return "clear" }
func (c *ClearCommand) Description() string                  { return "Clear the conversation history." }
func (c *ClearCommand) Type() CommandType                    { return CommandTypeLocal }
func (c *ClearCommand) IsEnabled(_ *ExecContext) bool        { return true }
func (c *ClearCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "__clear_history__", nil
}

// ─── /model ───────────────────────────────────────────────────────────────────

type ModelCommand struct{}

func (c *ModelCommand) Name() string                         { return "model" }
func (c *ModelCommand) Description() string                  { return "Show or set the active model. Usage: /model [name]" }
func (c *ModelCommand) Type() CommandType                    { return CommandTypeLocal }
func (c *ModelCommand) IsEnabled(_ *ExecContext) bool        { return true }
func (c *ModelCommand) Execute(_ context.Context, args []string, ectx *ExecContext) (string, error) {
	if len(args) == 0 {
		return "Current model: (check engine config)", nil
	}
	return fmt.Sprintf("Model set to: %s (restart engine to apply)", args[0]), nil
}

// ─── /compact ─────────────────────────────────────────────────────────────────

type CompactCommand struct{}

func (c *CompactCommand) Name() string                         { return "compact" }
func (c *CompactCommand) Description() string                  { return "Summarise and compact the conversation to free context window space." }
func (c *CompactCommand) Type() CommandType                    { return CommandTypeLocal }
func (c *CompactCommand) IsEnabled(_ *ExecContext) bool        { return true }
func (c *CompactCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "__compact__", nil
}

// ─── /cost ────────────────────────────────────────────────────────────────────

type CostCommand struct{}

func (c *CostCommand) Name() string                         { return "cost" }
func (c *CostCommand) Description() string                  { return "Show the accumulated cost for this session." }
func (c *CostCommand) Type() CommandType                    { return CommandTypeLocal }
func (c *CostCommand) IsEnabled(_ *ExecContext) bool        { return true }
func (c *CostCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	return "Use the HTTP API /api/v1/sessions/{id} to get cost information.", nil
}

// ─── /status ──────────────────────────────────────────────────────────────────

type StatusCommand struct{}

func (c *StatusCommand) Name() string                         { return "status" }
func (c *StatusCommand) Description() string                  { return "Show engine status." }
func (c *StatusCommand) Type() CommandType                    { return CommandTypeLocal }
func (c *StatusCommand) IsEnabled(_ *ExecContext) bool        { return true }
func (c *StatusCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "Status: OK", nil
	}
	return fmt.Sprintf("Status: OK\nSession: %s\nWorkDir: %s", ectx.SessionID, ectx.WorkDir), nil
}

// ─── Default registry with all built-in commands ─────────────────────────────

var defaultRegistry = func() *Registry {
	r := NewRegistry()
	r.Register(
		&HelpCommand{},
		&ClearCommand{},
		&ModelCommand{},
		&CompactCommand{},
		&CostCommand{},
		&StatusCommand{},
	)
	return r
}()

// Default returns the default built-in command registry.
func Default() *Registry { return defaultRegistry }
