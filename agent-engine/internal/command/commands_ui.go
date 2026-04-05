package command

import (
	"context"
	"strings"
)

// ─── /theme ──────────────────────────────────────────────────────────────────

// ThemeCommand changes the terminal theme.
// Aligned with claude-code-main commands/theme/index.ts (local-jsx).
type ThemeCommand struct{ BaseCommand }

func (c *ThemeCommand) Name() string                  { return "theme" }
func (c *ThemeCommand) Description() string           { return "Change the theme" }
func (c *ThemeCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ThemeCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ThemeCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	selected := ""
	if len(args) > 0 {
		selected = args[0]
	}
	return &InteractiveResult{
		Component: "theme",
		Data: map[string]interface{}{
			"current":  ectx.Theme,
			"selected": selected,
		},
	}, nil
}

// ─── /color ──────────────────────────────────────────────────────────────────

// ColorCommand configures color settings.
// Aligned with claude-code-main commands/color/index.ts (local).
type ColorCommand struct{ BaseCommand }

func (c *ColorCommand) Name() string                  { return "color" }
func (c *ColorCommand) Description() string           { return "Toggle color output" }
func (c *ColorCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ColorCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ColorCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "color",
	}, nil
}

// ─── /copy ───────────────────────────────────────────────────────────────────

// CopyCommand copies Claude's last response to clipboard.
// Aligned with claude-code-main commands/copy/index.ts (local-jsx).
type CopyCommand struct{ BaseCommand }

func (c *CopyCommand) Name() string { return "copy" }
func (c *CopyCommand) Description() string {
	return "Copy Claude's last response to clipboard (or /copy N for the Nth-latest)"
}
func (c *CopyCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *CopyCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *CopyCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	n := "1"
	if len(args) > 0 {
		n = args[0]
	}
	return &InteractiveResult{
		Component: "copy",
		Data:      map[string]interface{}{"n": n},
	}, nil
}

// ─── /export ─────────────────────────────────────────────────────────────────

// ExportCommand exports the current conversation to a file or clipboard.
// Aligned with claude-code-main commands/export/index.ts (local-jsx).
type ExportCommand struct{ BaseCommand }

func (c *ExportCommand) Name() string { return "export" }
func (c *ExportCommand) Description() string {
	return "Export the current conversation to a file or clipboard"
}
func (c *ExportCommand) ArgumentHint() string          { return "[filename]" }
func (c *ExportCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ExportCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ExportCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	filename := ""
	if len(args) > 0 {
		filename = strings.Join(args, " ")
	}
	return &InteractiveResult{
		Component: "export",
		Data:      map[string]interface{}{"filename": filename},
	}, nil
}

// ─── /keybindings ────────────────────────────────────────────────────────────

// KeybindingsCommand shows or configures keyboard shortcuts.
// Aligned with claude-code-main commands/keybindings/index.ts (local-jsx).
type KeybindingsCommand struct{ BaseCommand }

func (c *KeybindingsCommand) Name() string                  { return "keybindings" }
func (c *KeybindingsCommand) Aliases() []string             { return []string{"keys"} }
func (c *KeybindingsCommand) Description() string           { return "Show or configure keyboard shortcuts" }
func (c *KeybindingsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *KeybindingsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *KeybindingsCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "keybindings"}, nil
}

// ─── /output-style ───────────────────────────────────────────────────────────

// OutputStyleCommand configures the output style.
// Aligned with claude-code-main commands/output-style/index.ts (local-jsx).
type OutputStyleCommand struct{ BaseCommand }

func (c *OutputStyleCommand) Name() string                  { return "output-style" }
func (c *OutputStyleCommand) Description() string           { return "Configure output style preferences" }
func (c *OutputStyleCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *OutputStyleCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *OutputStyleCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "output-style"}, nil
}

// ─── /vim ────────────────────────────────────────────────────────────────────

// VimCommand toggles vim keybinding mode.
// Aligned with claude-code-main commands/vim/index.ts (local-jsx).
type VimCommand struct{ BaseCommand }

func (c *VimCommand) Name() string                  { return "vim" }
func (c *VimCommand) Description() string           { return "Toggle vim keybinding mode" }
func (c *VimCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *VimCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *VimCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "vim"}, nil
}

// ─── /rename ─────────────────────────────────────────────────────────────────

// RenameCommand renames the current conversation.
// Aligned with claude-code-main commands/rename/index.ts (local-jsx, immediate).
type RenameCommand struct{ BaseCommand }

func (c *RenameCommand) Name() string                  { return "rename" }
func (c *RenameCommand) Description() string           { return "Rename the current conversation" }
func (c *RenameCommand) ArgumentHint() string          { return "[name]" }
func (c *RenameCommand) IsImmediate() bool             { return true }
func (c *RenameCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *RenameCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *RenameCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	name := ""
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}
	return &InteractiveResult{
		Component: "rename",
		Data:      map[string]interface{}{"name": name},
	}, nil
}

// ─── /stickers ───────────────────────────────────────────────────────────────

// StickersCommand opens the sticker order page.
// Aligned with claude-code-main commands/stickers/index.ts (local).
type StickersCommand struct{ BaseCommand }

func (c *StickersCommand) Name() string                  { return "stickers" }
func (c *StickersCommand) Description() string           { return "Order openclaude-go stickers" }
func (c *StickersCommand) Type() CommandType             { return CommandTypeLocal }
func (c *StickersCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *StickersCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "Visit https://store.anthropic.com to order openclaude-go stickers!", nil
}

func init() {
	defaultRegistry.Register(
		&ThemeCommand{},
		&ColorCommand{},
		&CopyCommand{},
		&ExportCommand{},
		&KeybindingsCommand{},
		&OutputStyleCommand{},
		&VimCommand{},
		&RenameCommand{},
		&StickersCommand{},
	)
}
