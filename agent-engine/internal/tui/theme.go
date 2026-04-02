package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds all colour/style definitions for the TUI.
type Theme struct {
	User      lipgloss.Style
	Assistant lipgloss.Style
	System    lipgloss.Style
	Error     lipgloss.Style
	ToolUse   lipgloss.Style
	ToolResult lipgloss.Style

	StatusBar  lipgloss.Style
	Border     lipgloss.Style
	Spinner    lipgloss.Style
	Highlight  lipgloss.Style
	Dimmed     lipgloss.Style

	PermissionTitle  lipgloss.Style
	PermissionYes    lipgloss.Style
	PermissionNo     lipgloss.Style
}

// DefaultDarkTheme returns a dark-terminal-optimised theme.
func DefaultDarkTheme() Theme {
	return Theme{
		User: lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true),

		Assistant: lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")),

		System: lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Italic(true),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true),

		ToolUse: lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Italic(true),

		ToolResult: lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("250")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")),

		Spinner: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")),

		Highlight: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true),

		Dimmed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),

		PermissionTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true),

		PermissionYes: lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true),

		PermissionNo: lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true),
	}
}

// DefaultLightTheme returns a light-terminal-optimised theme.
func DefaultLightTheme() Theme {
	return Theme{
		User: lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")).
			Bold(true),

		Assistant: lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")),

		System: lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Italic(true),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true),

		ToolUse: lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Italic(true),

		ToolResult: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("254")).
			Foreground(lipgloss.Color("236")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("250")),

		Spinner: lipgloss.NewStyle().
			Foreground(lipgloss.Color("5")),

		Highlight: lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Bold(true),

		Dimmed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")),

		PermissionTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true),

		PermissionYes: lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			Bold(true),

		PermissionNo: lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true),
	}
}
