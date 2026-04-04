package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
)

// PermissionAnswerMsg is sent when the user answers a permission prompt.
type PermissionAnswerMsg struct {
	Granted bool
}

// PermissionModel is a modal confirmation dialog shown when a tool needs
// explicit user approval before running.
type PermissionModel struct {
	visible   bool
	toolName  string
	desc      string
	styles    themes.Styles
	themeData themes.Theme
	keymap    KeyMap
}

// NewPermissionModel creates an inactive permission dialog.
func NewPermissionModel(styles themes.Styles, km KeyMap) PermissionModel {
	return PermissionModel{styles: styles, keymap: km}
}

// NewPermissionModelWithTheme creates an inactive permission dialog with full theme data.
func NewPermissionModelWithTheme(styles themes.Styles, themeData themes.Theme, km KeyMap) PermissionModel {
	return PermissionModel{styles: styles, themeData: themeData, keymap: km}
}

// Ask activates the dialog for the given tool and description.
func (p *PermissionModel) Ask(toolName, desc string) {
	p.toolName = toolName
	p.desc = desc
	p.visible = true
}

// IsVisible reports whether the dialog is currently waiting for a response.
func (p PermissionModel) IsVisible() bool { return p.visible }

// Update handles key events while the dialog is visible.
func (p PermissionModel) Update(msg tea.Msg) (PermissionModel, tea.Cmd) {
	if !p.visible {
		return p, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch {
	case km.String() == "y" || km.String() == "Y":
		p.visible = false
		return p, func() tea.Msg { return PermissionAnswerMsg{Granted: true} }
	case km.String() == "n" || km.String() == "N" || km.String() == "esc":
		p.visible = false
		return p, func() tea.Msg { return PermissionAnswerMsg{Granted: false} }
	}
	return p, nil
}

// View renders the permission dialog as an overlay string.
// Returns "" when not visible.
// Matches claude-code-main PermissionDialog.tsx: top-only round border in permission color.
func (p PermissionModel) View() string {
	if !p.visible {
		return ""
	}

	// Resolve permission border color from themeData if available
	borderColor := p.styles.PermissionBorder.GetBorderTopForeground()
	if p.themeData.Permission != "" {
		borderColor = color.Resolve(p.themeData.Permission)
	}

	// Top-only round border with permission color (claude-code-main style)
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		MarginTop(1)

	var sb strings.Builder
	sb.WriteString(p.styles.PermissionTitle.Render("⚠  Permission Required") + "\n\n")
	sb.WriteString("  Tool: " + p.styles.Highlight.Render(p.toolName) + "\n")
	if p.desc != "" {
		sb.WriteString("  " + p.desc + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString("  " + p.styles.PermissionYes.Render("[y]") + " Allow  " +
		p.styles.PermissionNo.Render("[n]") + " Deny")

	return border.Render(sb.String())
}
