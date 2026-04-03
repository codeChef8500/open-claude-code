package permissionui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Response represents the user's answer to a permission prompt.
type Response int

const (
	ResponsePending     Response = iota
	ResponseAllow                // Allow this once
	ResponseDeny                 // Deny this once
	ResponseAlwaysAllow          // Allow and remember for session
	ResponseAlwaysDeny           // Deny and remember for session
)

// DialogTheme holds styles for the permission dialog.
type DialogTheme struct {
	Title     lipgloss.Style
	Body      lipgloss.Style
	Tool      lipgloss.Style
	Code      lipgloss.Style
	Allow     lipgloss.Style
	Deny      lipgloss.Style
	Dim       lipgloss.Style
	Border    lipgloss.Style
	Highlight lipgloss.Style
}

// DefaultDarkDialogTheme returns a dark-mode permission dialog theme.
func DefaultDarkDialogTheme() DialogTheme {
	return DialogTheme{
		Title: lipgloss.NewStyle().Foreground(lipgloss.Color("#b1b9f9")).Bold(true),
		Body:  lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		Tool:  lipgloss.NewStyle().Foreground(lipgloss.Color("#d77757")).Bold(true),
		Code:  lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		Allow: lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")).Bold(true),
		Deny:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b80")).Bold(true),
		Dim:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#b1b9f9")).
			BorderBottom(false).
			BorderLeft(false).
			BorderRight(false).
			MarginTop(1).
			PaddingLeft(1).PaddingRight(1),
		Highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true),
	}
}

// PermissionResponseMsg is sent when the user answers a permission dialog.
type PermissionResponseMsg struct {
	Response   Response
	ToolName   string
	ResponseCh chan<- bool
}

// PermissionDialog is an enhanced modal permission confirmation dialog.
type PermissionDialog struct {
	visible    bool
	toolName   string
	toolID     string
	detail     string
	command    string // for bash tools
	filePath   string // for file tools
	width      int
	responseCh chan<- bool
	theme      DialogTheme
	selected   int // 0 = Allow, 1 = Deny
}

// NewPermissionDialog creates a permission dialog.
func NewPermissionDialog(theme DialogTheme) *PermissionDialog {
	return &PermissionDialog{
		theme: theme,
		width: 60,
	}
}

// Show activates the dialog with the given tool details.
func (d *PermissionDialog) Show(toolName, toolID, detail string, responseCh chan<- bool) {
	d.visible = true
	d.toolName = toolName
	d.toolID = toolID
	d.detail = detail
	d.responseCh = responseCh
	d.selected = 0

	// Extract command or filePath from detail for better display
	d.command = ""
	d.filePath = ""
	switch toolName {
	case "Bash", "bash":
		d.command = detail
	case "Read", "read", "Edit", "edit", "Write", "write":
		d.filePath = detail
	}
}

// IsVisible reports whether the dialog is showing.
func (d *PermissionDialog) IsVisible() bool { return d.visible }

// SetWidth updates the dialog width.
func (d *PermissionDialog) SetWidth(w int) {
	d.width = w
	if d.width > 80 {
		d.width = 80
	}
	if d.width < 40 {
		d.width = 40
	}
}

// Update handles key events for the permission dialog.
func (d *PermissionDialog) Update(msg tea.Msg) (*PermissionDialog, tea.Cmd) {
	if !d.visible {
		return d, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return d, nil
	}

	switch {
	case km.String() == "y" || km.String() == "Y":
		return d, d.respond(ResponseAllow)

	case km.String() == "n" || km.String() == "N":
		return d, d.respond(ResponseDeny)

	case km.String() == "a" || km.String() == "A":
		return d, d.respond(ResponseAlwaysAllow)

	case km.String() == "d" || km.String() == "D":
		return d, d.respond(ResponseAlwaysDeny)

	case km.String() == "esc", km.Type == tea.KeyEscape:
		return d, d.respond(ResponseDeny)

	case km.String() == "tab", km.Type == tea.KeyTab:
		d.selected = (d.selected + 1) % 2

	case km.String() == "enter", km.Type == tea.KeyEnter:
		if d.selected == 0 {
			return d, d.respond(ResponseAllow)
		}
		return d, d.respond(ResponseDeny)

	case km.String() == "left", km.String() == "h":
		d.selected = 0

	case km.String() == "right", km.String() == "l":
		d.selected = 1
	}

	return d, nil
}

func (d *PermissionDialog) respond(r Response) tea.Cmd {
	d.visible = false
	ch := d.responseCh
	toolName := d.toolName

	allowed := r == ResponseAllow || r == ResponseAlwaysAllow

	return func() tea.Msg {
		if ch != nil {
			ch <- allowed
		}
		return PermissionResponseMsg{
			Response: r,
			ToolName: toolName,
		}
	}
}

// View renders the permission dialog.
func (d *PermissionDialog) View() string {
	if !d.visible {
		return ""
	}

	var sb strings.Builder

	// Title
	sb.WriteString(d.theme.Title.Render("⚠  Permission Required"))
	sb.WriteString("\n\n")

	// Tool name
	sb.WriteString(d.theme.Body.Render("Tool: "))
	sb.WriteString(d.theme.Tool.Render(d.toolName))
	sb.WriteString("\n")

	// Tool-specific detail
	if d.command != "" {
		sb.WriteString(d.theme.Body.Render("Command: "))
		sb.WriteString(d.theme.Code.Render(truncateForDialog(d.command, d.width-12)))
		sb.WriteString("\n")
	} else if d.filePath != "" {
		sb.WriteString(d.theme.Body.Render("File: "))
		sb.WriteString(d.theme.Code.Render(d.filePath))
		sb.WriteString("\n")
	} else if d.detail != "" {
		sb.WriteString(d.theme.Body.Render("Action: "))
		sb.WriteString(d.theme.Code.Render(truncateForDialog(d.detail, d.width-12)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Action buttons
	allowBtn := d.theme.Allow.Render("[y] Allow")
	denyBtn := d.theme.Deny.Render("[n] Deny")
	alwaysBtn := d.theme.Dim.Render("[a] Always Allow")
	alwaysDenyBtn := d.theme.Dim.Render("[d] Always Deny")

	if d.selected == 0 {
		allowBtn = d.theme.Highlight.Render("▸ ") + d.theme.Allow.Render("[y] Allow")
	} else {
		denyBtn = d.theme.Highlight.Render("▸ ") + d.theme.Deny.Render("[n] Deny")
	}

	sb.WriteString(fmt.Sprintf("  %s    %s\n", allowBtn, denyBtn))
	sb.WriteString(fmt.Sprintf("  %s  %s\n", alwaysBtn, alwaysDenyBtn))

	return d.theme.Border.Width(d.width).Render(sb.String())
}

func truncateForDialog(s string, maxLen int) string {
	// Replace newlines with ↵ for display
	s = strings.ReplaceAll(s, "\n", " ↵ ")
	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}
