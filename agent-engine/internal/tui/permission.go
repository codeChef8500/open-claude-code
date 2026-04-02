package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// PermissionAnswerMsg is sent when the user answers a permission prompt.
type PermissionAnswerMsg struct {
	Granted bool
}

// PermissionModel is a modal confirmation dialog shown when a tool needs
// explicit user approval before running.
type PermissionModel struct {
	visible  bool
	toolName string
	desc     string
	theme    Theme
	keymap   KeyMap
}

// NewPermissionModel creates an inactive permission dialog.
func NewPermissionModel(theme Theme, km KeyMap) PermissionModel {
	return PermissionModel{theme: theme, keymap: km}
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
func (p PermissionModel) View() string {
	if !p.visible {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(p.theme.PermissionTitle.Render("⚠  Permission Required") + "\n\n")
	sb.WriteString("Tool: " + p.theme.Highlight.Render(p.toolName) + "\n")
	if p.desc != "" {
		sb.WriteString("Action: " + p.desc + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(p.theme.PermissionYes.Render("[y]") + " Allow  " +
		p.theme.PermissionNo.Render("[n]") + " Deny")
	return p.theme.Border.Render(sb.String())
}
