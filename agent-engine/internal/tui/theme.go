package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds all colour/style definitions for the TUI.
type Theme struct {
	User       lipgloss.Style
	Assistant  lipgloss.Style
	System     lipgloss.Style
	Error      lipgloss.Style
	ToolUse    lipgloss.Style
	ToolResult lipgloss.Style

	StatusBar lipgloss.Style
	Border    lipgloss.Style
	Spinner   lipgloss.Style
	Highlight lipgloss.Style
	Dimmed    lipgloss.Style

	PermissionTitle lipgloss.Style
	PermissionYes   lipgloss.Style
	PermissionNo    lipgloss.Style
}

// DefaultDarkTheme returns a dark-terminal-optimised theme matching
// claude-code-main's dark color palette.
func DefaultDarkTheme() Theme {
	return Theme{
		User: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d77757")).
			Bold(true),

		Assistant: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d77757")),

		System: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6495ed")).
			Italic(true),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff6b80")).
			Bold(true),

		ToolUse: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d77757")),

		ToolResult: lipgloss.NewStyle().
			Faint(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("250")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#666666")),

		Spinner: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d77757")),

		Highlight: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true),

		Dimmed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")),

		PermissionTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b1b9f9")).
			Bold(true),

		PermissionYes: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4eba65")).
			Bold(true),

		PermissionNo: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff6b80")).
			Bold(true),
	}
}

// DefaultLightTheme returns a light-terminal-optimised theme matching
// claude-code-main's light color palette.
func DefaultLightTheme() Theme {
	return Theme{
		User: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b3522d")).
			Bold(true),

		Assistant: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b3522d")),

		System: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4169e1")).
			Italic(true),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e53e3e")).
			Bold(true),

		ToolUse: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b3522d")),

		ToolResult: lipgloss.NewStyle().
			Faint(true),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("254")).
			Foreground(lipgloss.Color("236")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#cccccc")),

		Spinner: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b3522d")),

		Highlight: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Bold(true),

		Dimmed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#999999")),

		PermissionTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6366f1")).
			Bold(true),

		PermissionYes: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#38a169")).
			Bold(true),

		PermissionNo: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e53e3e")).
			Bold(true),
	}
}
