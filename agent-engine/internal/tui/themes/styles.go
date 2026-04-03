package themes

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
)

// Styles holds pre-built lipgloss styles derived from a Theme.
// This bridges the new Theme color system with the existing TUI code
// that expects ready-made lipgloss.Style values.
type Styles struct {
	// Message roles
	User      lipgloss.Style
	Assistant lipgloss.Style
	System    lipgloss.Style
	Error     lipgloss.Style

	// Tool rendering
	ToolUse    lipgloss.Style
	ToolResult lipgloss.Style

	// UI chrome
	StatusBar lipgloss.Style
	Border    lipgloss.Style
	Spinner   lipgloss.Style
	Highlight lipgloss.Style
	Dimmed    lipgloss.Style

	// Permission dialog
	PermissionTitle  lipgloss.Style
	PermissionBorder lipgloss.Style
	PermissionYes    lipgloss.Style
	PermissionNo     lipgloss.Style

	// Diff
	DiffAdd lipgloss.Style
	DiffDel lipgloss.Style

	// Semantic
	Success lipgloss.Style
	Warning lipgloss.Style

	// Message dot (● prefix)
	Dot     lipgloss.Style
	DotUser lipgloss.Style

	// Response connector (⎿)
	Connector lipgloss.Style

	// Prompt
	PromptChar   lipgloss.Style
	PromptBorder lipgloss.Style
}

// BuildStyles creates lipgloss styles from a Theme's color values.
func BuildStyles(t Theme) Styles {
	c := func(s string) lipgloss.Color { return color.Resolve(s) }

	return Styles{
		User: lipgloss.NewStyle().
			Foreground(c(t.Claude)).
			Bold(true),

		Assistant: lipgloss.NewStyle().
			Foreground(c(t.Claude)),

		System: lipgloss.NewStyle().
			Foreground(c(t.Suggestion)).
			Italic(true),

		Error: lipgloss.NewStyle().
			Foreground(c(t.Error)).
			Bold(true),

		ToolUse: lipgloss.NewStyle().
			Foreground(c(t.Claude)).
			Italic(true),

		ToolResult: lipgloss.NewStyle().
			Foreground(c(t.Inactive)),

		StatusBar: lipgloss.NewStyle().
			Background(c(t.Subtle)).
			Foreground(c(t.Text)).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c(t.PromptBorder)),

		Spinner: lipgloss.NewStyle().
			Foreground(c(t.Claude)),

		Highlight: lipgloss.NewStyle().
			Foreground(c(t.Text)).
			Bold(true),

		Dimmed: lipgloss.NewStyle().
			Foreground(c(t.Inactive)),

		PermissionTitle: lipgloss.NewStyle().
			Foreground(c(t.Permission)).
			Bold(true),

		PermissionBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c(t.Permission)),

		PermissionYes: lipgloss.NewStyle().
			Foreground(c(t.Success)).
			Bold(true),

		PermissionNo: lipgloss.NewStyle().
			Foreground(c(t.Error)).
			Bold(true),

		DiffAdd: lipgloss.NewStyle().
			Foreground(c(t.DiffAddedWord)),

		DiffDel: lipgloss.NewStyle().
			Foreground(c(t.DiffRemovedWord)),

		Success: lipgloss.NewStyle().
			Foreground(c(t.Success)),

		Warning: lipgloss.NewStyle().
			Foreground(c(t.Warning)),

		Dot: lipgloss.NewStyle().
			Foreground(c(t.Claude)),

		DotUser: lipgloss.NewStyle().
			Foreground(c(t.Claude)).
			Bold(true),

		Connector: lipgloss.NewStyle().
			Faint(true),

		PromptChar: lipgloss.NewStyle().
			Foreground(c(t.PromptBorder)),

		PromptBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c(t.PromptBorder)),
	}
}

// StatusLineStyles builds styles specifically for the status line.
type StatusLineStyles struct {
	Background lipgloss.Style
	Model      lipgloss.Style
	Cost       lipgloss.Style
	Context    lipgloss.Style
	Mode       lipgloss.Style
	Path       lipgloss.Style
	Dim        lipgloss.Style
	Warning    lipgloss.Style
	Active     lipgloss.Style
}

// BuildStatusLineStyles creates status line styles from a Theme.
func BuildStatusLineStyles(t Theme) StatusLineStyles {
	c := func(s string) lipgloss.Color { return color.Resolve(s) }
	bg := lipgloss.NewStyle().Background(c(t.Subtle)).Foreground(c(t.Text))

	return StatusLineStyles{
		Background: bg.Padding(0, 1),
		Model:      bg.Foreground(c(t.Text)).Bold(true),
		Cost:       bg.Foreground(c(t.Success)),
		Context:    bg.Foreground(c(t.Suggestion)),
		Mode:       bg.Foreground(c(t.Warning)),
		Path:       bg.Foreground(c(t.Inactive)),
		Dim:        bg.Foreground(c(t.Inactive)),
		Warning:    bg.Foreground(c(t.Error)).Bold(true),
		Active:     bg.Foreground(c(t.Claude)),
	}
}

// ToolUIStyles builds styles for tool-specific rendering.
type ToolUIStyles struct {
	ToolIcon lipgloss.Style
	TreeConn lipgloss.Style
	Code     lipgloss.Style
	FilePath lipgloss.Style
	Dim      lipgloss.Style
	Success  lipgloss.Style
	Error    lipgloss.Style
	Output   lipgloss.Style
	DiffAdd  lipgloss.Style
	DiffDel  lipgloss.Style
	DiffCtx  lipgloss.Style
	DiffHdr  lipgloss.Style
}

// BuildToolUIStyles creates tool UI styles from a Theme.
func BuildToolUIStyles(t Theme) ToolUIStyles {
	c := func(s string) lipgloss.Color { return color.Resolve(s) }

	return ToolUIStyles{
		ToolIcon: lipgloss.NewStyle().Foreground(c(t.Claude)).Italic(true),
		TreeConn: lipgloss.NewStyle().Foreground(c(t.Inactive)),
		Code:     lipgloss.NewStyle().Foreground(c(t.Text)),
		FilePath: lipgloss.NewStyle().Foreground(c(t.Suggestion)).Underline(true),
		Dim:      lipgloss.NewStyle().Foreground(c(t.Inactive)),
		Success:  lipgloss.NewStyle().Foreground(c(t.Success)),
		Error:    lipgloss.NewStyle().Foreground(c(t.Error)),
		Output:   lipgloss.NewStyle().Foreground(c(t.Inactive)),
		DiffAdd:  lipgloss.NewStyle().Foreground(c(t.DiffAddedWord)),
		DiffDel:  lipgloss.NewStyle().Foreground(c(t.DiffRemovedWord)),
		DiffCtx:  lipgloss.NewStyle().Foreground(c(t.Inactive)),
		DiffHdr:  lipgloss.NewStyle().Foreground(c(t.Suggestion)).Bold(true),
	}
}
