package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /verbose ─────────────────────────────────────────────────────────────────

// VerboseCommand toggles verbose/debug output.
type VerboseCommand struct{ BaseCommand }

func (c *VerboseCommand) Name() string { return "verbose" }
func (c *VerboseCommand) Description() string {
	return "Toggle verbose output. Usage: /verbose [on|off]"
}
func (c *VerboseCommand) Type() CommandType             { return CommandTypeLocal }
func (c *VerboseCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *VerboseCommand) Execute(_ context.Context, args []string, ectx *ExecContext) (string, error) {
	if len(args) == 0 {
		// Toggle.
		if ectx != nil {
			ectx.Verbose = !ectx.Verbose
			return fmt.Sprintf("Verbose: %v", ectx.Verbose), nil
		}
		return "Verbose: unknown (no session context)", nil
	}
	switch strings.ToLower(args[0]) {
	case "on", "true", "1":
		if ectx != nil {
			ectx.Verbose = true
		}
		return "Verbose: on", nil
	case "off", "false", "0":
		if ectx != nil {
			ectx.Verbose = false
		}
		return "Verbose: off", nil
	}
	return "Usage: /verbose [on|off]", nil
}

// ─── /plan ────────────────────────────────────────────────────────────────────

// PlanCommand toggles plan-only mode.
type PlanCommand struct{ BaseCommand }

func (c *PlanCommand) Name() string { return "plan" }
func (c *PlanCommand) Description() string {
	return "Toggle plan mode (agent proposes before acting). Usage: /plan [on|off]"
}
func (c *PlanCommand) Type() CommandType             { return CommandTypeLocal }
func (c *PlanCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *PlanCommand) Execute(_ context.Context, args []string, ectx *ExecContext) (string, error) {
	if len(args) == 0 {
		return "Usage: /plan [on|off]", nil
	}
	switch strings.ToLower(args[0]) {
	case "on", "true", "1":
		return "__plan_mode__:on", nil
	case "off", "false", "0":
		return "__plan_mode__:off", nil
	}
	return "Usage: /plan [on|off]", nil
}

// ─── Register ─────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.Register(
		&VerboseCommand{},
		&PlanCommand{},
	)
}
