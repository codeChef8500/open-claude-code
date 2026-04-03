package designsystem

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
)

// StatusIconType identifies the kind of status icon to display.
type StatusIconType string

const (
	StatusSuccess StatusIconType = "success" // ✓ green
	StatusError   StatusIconType = "error"   // ✗ red
	StatusWarning StatusIconType = "warning" // ⚠ yellow
	StatusInfo    StatusIconType = "info"    // ℹ blue
	StatusPending StatusIconType = "pending" // ○ dim
	StatusLoading StatusIconType = "loading" // … dim
)

type statusConfig struct {
	icon       string
	themeColor string // empty means use dim
}

var statusConfigs = map[StatusIconType]statusConfig{
	StatusSuccess: {icon: "✓"},
	StatusError:   {icon: "✗"},
	StatusWarning: {icon: "⚠"},
	StatusInfo:    {icon: "ℹ"},
	StatusPending: {icon: "○"},
	StatusLoading: {icon: "…"},
}

// RenderStatusIcon renders a status indicator icon with the appropriate color.
// successColor, errorColor, warningColor, infoColor are theme color strings.
// If withSpace is true, a trailing space is appended.
func RenderStatusIcon(status StatusIconType, successColor, errorColor, warningColor, infoColor string, withSpace bool) string {
	cfg, ok := statusConfigs[status]
	if !ok {
		cfg = statusConfigs[StatusPending]
	}

	var style lipgloss.Style
	switch status {
	case StatusSuccess:
		style = lipgloss.NewStyle().Foreground(color.Resolve(successColor))
	case StatusError:
		style = lipgloss.NewStyle().Foreground(color.Resolve(errorColor))
	case StatusWarning:
		style = lipgloss.NewStyle().Foreground(color.Resolve(warningColor))
	case StatusInfo:
		style = lipgloss.NewStyle().Foreground(color.Resolve(infoColor))
	default:
		style = lipgloss.NewStyle().Faint(true)
	}

	result := style.Render(cfg.icon)
	if withSpace {
		result += " "
	}
	return result
}

// RenderStatusIconSimple renders a status icon using default ANSI colors.
func RenderStatusIconSimple(status StatusIconType, withSpace bool) string {
	return RenderStatusIcon(status,
		"rgb(78,186,101)",  // success green
		"rgb(255,107,128)", // error red
		"rgb(255,193,7)",   // warning yellow
		"rgb(100,149,237)", // info blue
		withSpace,
	)
}
