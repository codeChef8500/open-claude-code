package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /help ────────────────────────────────────────────────────────────────────

type HelpCommand struct{ BaseCommand }

func (c *HelpCommand) Name() string                  { return "help" }
func (c *HelpCommand) Description() string           { return "Show available slash commands." }
func (c *HelpCommand) Type() CommandType             { return CommandTypeLocal }
func (c *HelpCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *HelpCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "", nil
	}
	lines := []string{"Available commands:"}
	for _, cmd := range defaultRegistry.VisibleFor(ectx, AvailabilityConsole) {
		desc := FormatDescriptionWithSource(cmd)
		if hint := cmd.ArgumentHint(); hint != "" {
			lines = append(lines, fmt.Sprintf("  /%s %s — %s", cmd.Name(), hint, desc))
			continue
		}
		lines = append(lines, fmt.Sprintf("  /%s — %s", cmd.Name(), desc))
	}
	return strings.Join(lines, "\n"), nil
}

// ─── /clear ───────────────────────────────────────────────────────────────────

type ClearCommand struct{ BaseCommand }

func (c *ClearCommand) Name() string                  { return "clear" }
func (c *ClearCommand) Description() string           { return "Clear the conversation history." }
func (c *ClearCommand) Type() CommandType             { return CommandTypeLocal }
func (c *ClearCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ClearCommand) Execute(ctx context.Context, _ []string, ectx *ExecContext) (string, error) {
	return ClearConversation(ctx, ectx)
}

// ─── /model ───────────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/model/index.ts (local-jsx).
// Dynamic description shows current model. Interactive picker in TUI.

type ModelCommand struct{ BaseCommand }

func (c *ModelCommand) Name() string { return "model" }
func (c *ModelCommand) Description() string {
	return "Set the AI model"
}
func (c *ModelCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ModelCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ModelCommand) ArgumentHint() string          { return "[model-name]" }

// DynamicDescription returns a description that includes the current model.
// Called by TUI to show "Set the AI model (currently claude-sonnet-4-20250514)".
func (c *ModelCommand) DynamicDescription(ectx *ExecContext) string {
	if ectx != nil && ectx.Model != "" {
		return fmt.Sprintf("Set the AI model (currently %s)", ectx.Model)
	}
	return "Set the AI model"
}

func (c *ModelCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &ModelPickerData{}
	if ectx != nil {
		data.CurrentModel = ectx.Model
	}
	if len(args) > 0 {
		data.SelectedModel = args[0]
	}
	return &InteractiveResult{
		Component: "model",
		Data:      data,
	}, nil
}

// ModelPickerData is the structured data for the model picker TUI component.
type ModelPickerData struct {
	CurrentModel  string `json:"current_model"`
	SelectedModel string `json:"selected_model,omitempty"`
}

// ─── /compact ─────────────────────────────────────────────────────────────────

type CompactCommand struct{ BaseCommand }

func (c *CompactCommand) Name() string { return "compact" }
func (c *CompactCommand) Description() string {
	return "Summarise and compact the conversation to free context window space."
}
func (c *CompactCommand) Type() CommandType             { return CommandTypeLocal }
func (c *CompactCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *CompactCommand) Execute(ctx context.Context, args []string, ectx *ExecContext) (string, error) {
	return CompactConversation(ctx, args, ectx)
}

// CostCommand stub removed — replaced by DeepCostCommand in cost_impl.go.

// ─── /status ──────────────────────────────────────────────────────────────────

type StatusCommand struct{ BaseCommand }

func (c *StatusCommand) Name() string                  { return "status" }
func (c *StatusCommand) Description() string           { return "Show engine status." }
func (c *StatusCommand) Type() CommandType             { return CommandTypeLocal }
func (c *StatusCommand) IsEnabled(_ *ExecContext) bool { return true }
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
		&StatusCommand{},
	)
	return r
}()

// Default returns the default built-in command registry.
func Default() *Registry { return defaultRegistry }
