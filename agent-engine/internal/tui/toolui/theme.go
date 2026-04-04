package toolui

import "github.com/charmbracelet/lipgloss"

// ToolUITheme holds styles for tool-specific rendering.
type ToolUITheme struct {
	ToolIcon    lipgloss.Style
	TreeConn    lipgloss.Style
	Code        lipgloss.Style
	Output      lipgloss.Style
	Dim         lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	FilePath    lipgloss.Style
	DiffAdd     lipgloss.Style
	DiffDel     lipgloss.Style
	DiffCtx     lipgloss.Style
	DiffHdr     lipgloss.Style
	DiffAddWord lipgloss.Style // word-level highlight: green bg
	DiffDelWord lipgloss.Style // word-level highlight: red bg
}

// DefaultDarkToolUITheme returns a dark-mode tool UI theme.
func DefaultDarkToolUITheme() ToolUITheme {
	return ToolUITheme{
		ToolIcon:    lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Italic(true),
		TreeConn:    lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Code:        lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		Output:      lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		Dim:         lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		Success:     lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		FilePath:    lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Underline(true),
		DiffAdd:     lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		DiffDel:     lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		DiffCtx:     lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		DiffHdr:     lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true),
		DiffAddWord: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("22")),
		DiffDelWord: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("52")),
	}
}

// DefaultLightToolUITheme returns a light-mode tool UI theme.
func DefaultLightToolUITheme() ToolUITheme {
	return ToolUITheme{
		ToolIcon:    lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Italic(true),
		TreeConn:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Code:        lipgloss.NewStyle().Foreground(lipgloss.Color("0")),
		Output:      lipgloss.NewStyle().Foreground(lipgloss.Color("236")),
		Dim:         lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		Success:     lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		FilePath:    lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Underline(true),
		DiffAdd:     lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		DiffDel:     lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		DiffCtx:     lipgloss.NewStyle().Foreground(lipgloss.Color("236")),
		DiffHdr:     lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		DiffAddWord: lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("157")),
		DiffDelWord: lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("217")),
	}
}
