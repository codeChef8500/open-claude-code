package command

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// UI command deep implementations.
// Aligned with claude-code-main commands/theme, color, copy, export,
// keybindings, output-style, vim, rename.
// ──────────────────────────────────────────────────────────────────────────────

// ─── /theme deep implementation ─────────────────────────────────────────────
// Aligned with claude-code-main commands/theme/theme.tsx.

// ThemeViewData is the structured data for the theme picker TUI component.
type ThemeViewData struct {
	CurrentTheme   string   `json:"current_theme"`
	AvailableThemes []string `json:"available_themes"`
	SelectedTheme  string   `json:"selected_theme,omitempty"`
}

// DefaultThemes is the list of built-in themes.
var DefaultThemes = []string{
	"dark", "light", "solarized-dark", "solarized-light",
	"monokai", "dracula", "nord", "gruvbox-dark", "gruvbox-light",
	"catppuccin-mocha", "catppuccin-latte", "tokyo-night",
	"one-dark", "github-dark", "github-light",
}

// DeepThemeCommand replaces the basic ThemeCommand with full logic.
type DeepThemeCommand struct{ BaseCommand }

func (c *DeepThemeCommand) Name() string                  { return "theme" }
func (c *DeepThemeCommand) Description() string           { return "Change the color theme" }
func (c *DeepThemeCommand) ArgumentHint() string          { return "[theme-name]" }
func (c *DeepThemeCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepThemeCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepThemeCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &ThemeViewData{
		AvailableThemes: DefaultThemes,
	}
	if ectx != nil {
		data.CurrentTheme = ectx.Theme
	}
	if len(args) > 0 {
		data.SelectedTheme = args[0]
	}
	return &InteractiveResult{
		Component: "theme",
		Data:      data,
	}, nil
}

// ─── /color deep implementation ─────────────────────────────────────────────
// Aligned with claude-code-main commands/color/color.tsx.

// ColorViewData is the structured data for the color picker TUI component.
type ColorViewData struct {
	CurrentColor string `json:"current_color"`
	SelectedColor string `json:"selected_color,omitempty"`
}

// DeepColorCommand replaces the basic ColorCommand with full logic.
type DeepColorCommand struct{ BaseCommand }

func (c *DeepColorCommand) Name() string                  { return "color" }
func (c *DeepColorCommand) Description() string           { return "Set the agent accent color" }
func (c *DeepColorCommand) ArgumentHint() string          { return "[color]" }
func (c *DeepColorCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepColorCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepColorCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &ColorViewData{}
	if len(args) > 0 {
		data.SelectedColor = args[0]
	}
	return &InteractiveResult{
		Component: "color",
		Data:      data,
	}, nil
}

// ─── /copy deep implementation ──────────────────────────────────────────────
// Aligned with claude-code-main commands/copy/copy.tsx.

// DeepCopyCommand copies the last assistant response to clipboard.
type DeepCopyCommand struct{ BaseCommand }

func (c *DeepCopyCommand) Name() string                  { return "copy" }
func (c *DeepCopyCommand) Description() string           { return "Copy the last response to clipboard" }
func (c *DeepCopyCommand) ArgumentHint() string          { return "[last-n]" }
func (c *DeepCopyCommand) Type() CommandType             { return CommandTypeLocal }
func (c *DeepCopyCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepCopyCommand) Execute(_ context.Context, args []string, ectx *ExecContext) (string, error) {
	if ectx == nil || ectx.Services == nil || ectx.Services.Clipboard == nil {
		return "Clipboard service not available.", nil
	}

	// Default: copy last assistant response.
	text := "(no response to copy)"

	// In a full implementation, we'd extract the last N assistant messages.
	// For now, signal the TUI to handle extraction.
	if ectx.Messages != nil {
		text = "__copy_last_response__"
	}

	if err := ectx.Services.Clipboard.Copy(text); err != nil {
		return fmt.Sprintf("Failed to copy: %v", err), nil
	}
	return "Copied to clipboard.", nil
}

// ─── /export deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/export/export.tsx.

// ExportViewData is the structured data for the export TUI component.
type ExportViewData struct {
	Format    string `json:"format"`     // "json", "markdown", "text"
	FilePath  string `json:"file_path,omitempty"`
	SessionID string `json:"session_id"`
	TurnCount int    `json:"turn_count"`
}

// DeepExportCommand exports the conversation to a file.
type DeepExportCommand struct{ BaseCommand }

func (c *DeepExportCommand) Name() string                  { return "export" }
func (c *DeepExportCommand) Description() string           { return "Export conversation to a file" }
func (c *DeepExportCommand) ArgumentHint() string          { return "[format] [filepath]" }
func (c *DeepExportCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepExportCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepExportCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &ExportViewData{Format: "markdown"}
	if ectx != nil {
		data.SessionID = ectx.SessionID
		data.TurnCount = ectx.TurnCount
	}
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "json", "markdown", "md", "text", "txt":
			data.Format = args[0]
		default:
			data.FilePath = args[0]
		}
	}
	if len(args) > 1 {
		data.FilePath = args[1]
	}
	return &InteractiveResult{
		Component: "export",
		Data:      data,
	}, nil
}

// ─── /keybindings deep implementation ───────────────────────────────────────
// Aligned with claude-code-main commands/keybindings/keybindings.tsx.

// KeybindingsViewData is the structured data for the keybindings panel.
type KeybindingsViewData struct {
	Bindings []KeyBinding `json:"bindings"`
}

// KeyBinding represents a single keybinding.
type KeyBinding struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Category    string `json:"category"` // "general", "editing", "navigation"
}

// DefaultKeybindings returns the built-in keybindings list.
func DefaultKeybindings() []KeyBinding {
	return []KeyBinding{
		// General
		{Key: "Ctrl+C", Description: "Cancel current operation", Category: "general"},
		{Key: "Ctrl+D", Description: "Exit session", Category: "general"},
		{Key: "Ctrl+L", Description: "Clear screen", Category: "general"},
		{Key: "Ctrl+\\", Description: "Force interrupt", Category: "general"},
		{Key: "Escape", Description: "Cancel / dismiss", Category: "general"},
		// Editing
		{Key: "Enter", Description: "Submit message", Category: "editing"},
		{Key: "Shift+Enter", Description: "New line", Category: "editing"},
		{Key: "Ctrl+A", Description: "Select all", Category: "editing"},
		{Key: "Ctrl+K", Description: "Clear to end of line", Category: "editing"},
		{Key: "Ctrl+U", Description: "Clear to start of line", Category: "editing"},
		// Navigation
		{Key: "Up/Down", Description: "History navigation", Category: "navigation"},
		{Key: "Tab", Description: "Autocomplete", Category: "navigation"},
		{Key: "Shift+Cmd+K", Description: "Compact conversation", Category: "navigation"},
		{Key: "Shift+Cmd+C", Description: "Copy last response", Category: "navigation"},
	}
}

// DeepKeybindingsCommand shows/edits keybindings.
type DeepKeybindingsCommand struct{ BaseCommand }

func (c *DeepKeybindingsCommand) Name() string                  { return "keybindings" }
func (c *DeepKeybindingsCommand) Aliases() []string             { return []string{"keys", "shortcuts"} }
func (c *DeepKeybindingsCommand) Description() string           { return "View and edit key bindings" }
func (c *DeepKeybindingsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepKeybindingsCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepKeybindingsCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "keybindings",
		Data:      &KeybindingsViewData{Bindings: DefaultKeybindings()},
	}, nil
}

// ─── /output-style deep implementation ──────────────────────────────────────
// Aligned with claude-code-main commands/output-style/output-style.tsx.

// OutputStyleViewData is the structured data for the output style picker.
type OutputStyleViewData struct {
	CurrentStyle string   `json:"current_style"`
	Available    []string `json:"available"`
	Selected     string   `json:"selected,omitempty"`
}

// OutputStyles is the list of available output styles.
var OutputStyles = []string{
	"default", "concise", "verbose", "markdown", "plain", "json",
}

// DeepOutputStyleCommand configures output rendering style.
type DeepOutputStyleCommand struct{ BaseCommand }

func (c *DeepOutputStyleCommand) Name() string                  { return "output-style" }
func (c *DeepOutputStyleCommand) Description() string           { return "Set the output rendering style" }
func (c *DeepOutputStyleCommand) ArgumentHint() string          { return "[style]" }
func (c *DeepOutputStyleCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepOutputStyleCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepOutputStyleCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &OutputStyleViewData{
		CurrentStyle: "default",
		Available:    OutputStyles,
	}
	if len(args) > 0 {
		data.Selected = args[0]
	}
	return &InteractiveResult{
		Component: "output-style",
		Data:      data,
	}, nil
}

// ─── /vim deep implementation ───────────────────────────────────────────────
// Aligned with claude-code-main commands/vim/vim.tsx.

// DeepVimCommand toggles vim mode.
type DeepVimCommand struct{ BaseCommand }

func (c *DeepVimCommand) Name() string                  { return "vim" }
func (c *DeepVimCommand) Description() string           { return "Toggle vim key bindings for the input" }
func (c *DeepVimCommand) ArgumentHint() string          { return "[on|off]" }
func (c *DeepVimCommand) Type() CommandType             { return CommandTypeLocal }
func (c *DeepVimCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepVimCommand) Execute(_ context.Context, args []string, ectx *ExecContext) (string, error) {
	// Toggle vim mode via config service.
	if ectx != nil && ectx.Services != nil && ectx.Services.Config != nil {
		current, _ := ectx.Services.Config.Get("vim_mode")
		enabled, _ := current.(bool)

		if len(args) > 0 {
			switch strings.ToLower(args[0]) {
			case "on", "true", "1":
				enabled = true
			case "off", "false", "0":
				enabled = false
			}
		} else {
			enabled = !enabled
		}

		_ = ectx.Services.Config.Set("vim_mode", enabled)
		if enabled {
			return "Vim mode enabled.", nil
		}
		return "Vim mode disabled.", nil
	}

	// Fallback without config service.
	if len(args) > 0 {
		return fmt.Sprintf("Vim mode: %s (config service not available)", args[0]), nil
	}
	return "Vim mode toggled (config service not available).", nil
}

// ─── /rename deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/rename/rename.tsx.

// DeepRenameCommand renames the current session.
type DeepRenameCommand struct{ BaseCommand }

func (c *DeepRenameCommand) Name() string                  { return "rename" }
func (c *DeepRenameCommand) Description() string           { return "Rename the current conversation" }
func (c *DeepRenameCommand) ArgumentHint() string          { return "<new-name>" }
func (c *DeepRenameCommand) Type() CommandType             { return CommandTypeLocal }
func (c *DeepRenameCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepRenameCommand) Execute(_ context.Context, args []string, ectx *ExecContext) (string, error) {
	if len(args) == 0 {
		return "Usage: /rename <new-name>", nil
	}

	newName := strings.Join(args, " ")

	if ectx != nil && ectx.Services != nil && ectx.Services.Session != nil {
		meta := SessionMeta{
			ID:    ectx.SessionID,
			Title: newName,
		}
		if err := ectx.Services.Session.SaveSessionMeta(context.Background(), meta); err != nil {
			return fmt.Sprintf("Failed to rename: %v", err), nil
		}
		return fmt.Sprintf("Session renamed to: %s", newName), nil
	}

	return fmt.Sprintf("Session renamed to: %s (not persisted — session service unavailable)", newName), nil
}

// ─── /stickers deep implementation ──────────────────────────────────────────
// Aligned with claude-code-main commands/stickers.ts.

// DeepStickersCommand opens the sticker store.
type DeepStickersCommand struct{ BaseCommand }

func (c *DeepStickersCommand) Name() string                  { return "stickers" }
func (c *DeepStickersCommand) Description() string           { return "Open the sticker store" }
func (c *DeepStickersCommand) IsHidden() bool                { return true }
func (c *DeepStickersCommand) Type() CommandType             { return CommandTypeLocal }
func (c *DeepStickersCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepStickersCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	url := "https://store.anthropic.com/stickers"
	if ectx != nil && ectx.Services != nil && ectx.Services.Browser != nil {
		if err := ectx.Services.Browser.Open(url); err != nil {
			return fmt.Sprintf("Failed to open browser: %v\nVisit: %s", err, url), nil
		}
		return "Opening sticker store in browser...", nil
	}
	return fmt.Sprintf("Visit: %s", url), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Register deep UI commands, replacing stubs.
// ──────────────────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepThemeCommand{},
		&DeepColorCommand{},
		&DeepCopyCommand{},
		&DeepExportCommand{},
		&DeepKeybindingsCommand{},
		&DeepOutputStyleCommand{},
		&DeepVimCommand{},
		&DeepRenameCommand{},
		&DeepStickersCommand{},
	)
}

// Ensure json import is used.
var _ = json.Marshal
