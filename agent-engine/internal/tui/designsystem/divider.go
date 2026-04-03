package designsystem

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
)

// RenderDivider renders a horizontal divider line, matching claude-code-main's
// Divider component. If title is non-empty, it is centered in the line:
//
//	──────── Title ────────
//
// If title is empty, a plain line is rendered:
//
//	────────────────────────
//
// The char defaults to '─' (U+2500). If themeColor is empty, dimColor is used.
func RenderDivider(width int, title string, themeColor string) string {
	return RenderDividerChar(width, title, themeColor, "─")
}

// RenderDividerChar is like RenderDivider but allows specifying a custom
// line character.
func RenderDividerChar(width int, title string, themeColor string, char string) string {
	if width <= 0 {
		return ""
	}

	var style lipgloss.Style
	if themeColor != "" {
		style = lipgloss.NewStyle().Foreground(color.Resolve(themeColor))
	} else {
		style = lipgloss.NewStyle().Faint(true)
	}

	if title == "" {
		return style.Render(strings.Repeat(char, width))
	}

	// Title mode: ─── Title ───
	titleWidth := lipgloss.Width(title) + 2 // 1 space on each side
	sideWidth := width - titleWidth
	if sideWidth < 2 {
		return style.Render(title)
	}
	leftWidth := sideWidth / 2
	rightWidth := sideWidth - leftWidth

	dimTitle := lipgloss.NewStyle().Faint(true).Render(title)

	return style.Render(strings.Repeat(char, leftWidth)) +
		" " + dimTitle + " " +
		style.Render(strings.Repeat(char, rightWidth))
}
