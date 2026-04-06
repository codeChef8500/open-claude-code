package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /verbose ─────────────────────────────────────────────────────────────────

// VerboseCommand toggles verbose/debug output.
type VerboseCommand struct{ BaseCommand }

func (c *VerboseCommand) Name() string                  { return "verbose" }
func (c *VerboseCommand) ArgumentHint() string          { return "[on|off]" }
func (c *VerboseCommand) Description() string           { return "Toggle verbose output" }
func (c *VerboseCommand) Type() CommandType             { return CommandTypeLocal }
func (c *VerboseCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *VerboseCommand) Execute(_ context.Context, args []string, ectx *ExecContext) (string, error) {
	if len(args) == 0 {
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
// Aligned with claude-code-main commands/plan/index.ts (local-jsx).

// PlanCommand enables plan mode or views current session plan.
type PlanCommand struct{ BaseCommand }

func (c *PlanCommand) Name() string         { return "plan" }
func (c *PlanCommand) ArgumentHint() string { return "[message]" }
func (c *PlanCommand) Description() string {
	return "Toggle plan mode or view current session plan"
}
func (c *PlanCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *PlanCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *PlanCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := map[string]interface{}{}
	if ectx != nil {
		data["planModeActive"] = ectx.PlanModeActive
		if ectx.PlanModeActive {
			data["fallback_text"] = "Plan mode: ON — will plan without executing."
		} else {
			data["fallback_text"] = "Plan mode: OFF — normal execution."
		}
	}
	if len(args) > 0 {
		data["message"] = strings.Join(args, " ")
		data["fallback_text"] = "Plan mode activated with message: " + strings.Join(args, " ")
	}
	return &InteractiveResult{
		Component: "plan",
		Data:      data,
	}, nil
}

// ─── /fast ───────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/fast/index.ts (local-jsx).

// FastCommand toggles fast mode.
type FastCommand struct{ BaseCommand }

func (c *FastCommand) Name() string                  { return "fast" }
func (c *FastCommand) Description() string           { return "Toggle fast mode (use smaller, faster model)" }
func (c *FastCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *FastCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *FastCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := map[string]interface{}{}
	if ectx != nil {
		data["fastMode"] = ectx.FastMode
		if ectx.FastMode {
			data["fallback_text"] = "Fast mode: ON — using smaller, faster model for simple tasks."
		} else {
			data["fallback_text"] = "Fast mode: OFF — using default model."
		}
	}
	return &InteractiveResult{
		Component: "fast",
		Data:      data,
	}, nil
}

// ─── /effort ─────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/effort/index.ts (local-jsx, immediate).

// EffortCommand sets the effort level for model usage.
type EffortCommand struct{ BaseCommand }

func (c *EffortCommand) Name() string                  { return "effort" }
func (c *EffortCommand) ArgumentHint() string          { return "[low|medium|high|max|auto]" }
func (c *EffortCommand) Description() string           { return "Set effort level for model usage" }
func (c *EffortCommand) IsImmediate() bool             { return true }
func (c *EffortCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *EffortCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *EffortCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := map[string]interface{}{}
	current := "medium"
	if ectx != nil {
		if ectx.EffortLevel != "" {
			current = ectx.EffortLevel
		}
		data["current"] = current
	}
	if len(args) > 0 {
		selected := strings.ToLower(args[0])
		data["selected"] = selected
		data["fallback_text"] = fmt.Sprintf("Effort level set to: %s", selected)
	} else {
		data["fallback_text"] = fmt.Sprintf("Effort level: %s\nUsage: /effort [low|medium|high|max|auto]", current)
	}
	return &InteractiveResult{
		Component: "effort",
		Data:      data,
	}, nil
}

// ─── Register ─────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.Register(
		&VerboseCommand{},
		&PlanCommand{},
		&FastCommand{},
		&EffortCommand{},
	)
}
