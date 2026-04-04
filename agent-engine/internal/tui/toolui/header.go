package toolui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/figures"
)

// ResponsePrefix is the dim "⎿" connector used for tool results,
// matching claude-code-main's MessageResponse component.
const ResponsePrefix = "  ⎿  "

// RenderToolHeader renders a unified tool header line matching claude-code:
//
//	● ToolName (params)
//
// The dot color is determined by the DotState. The tool name is bold.
// If params is empty, the parenthesized portion is omitted.
func RenderToolHeader(dotStr, toolName, params string, theme ToolUITheme) string {
	var sb strings.Builder

	// Dot (pre-colored by caller via ToolDot.View())
	sb.WriteString(dotStr)

	// Tool name — bold
	nameStyle := theme.ToolIcon.Bold(true).Italic(false)
	sb.WriteString(nameStyle.Render(toolName))

	// Params — normal weight, parenthesized
	if params != "" {
		sb.WriteString(" ")
		sb.WriteString(theme.Dim.Render("(" + params + ")"))
	}

	return sb.String()
}

// RenderToolHeaderSimple renders a header with the default dot glyph in a given color.
//
//	● ToolName (params)
func RenderToolHeaderSimple(toolName, params string, dotColor lipgloss.Color, theme ToolUITheme) string {
	dotStyle := lipgloss.NewStyle().Foreground(dotColor)
	dot := dotStyle.Render(figures.BlackCircle()) + " "
	return RenderToolHeader(dot, toolName, params, theme)
}

// RenderResponseLine renders a tool result line with the ⎿ connector prefix.
//
//	  ⎿  content text here
func RenderResponseLine(content string, theme ToolUITheme) string {
	return theme.TreeConn.Render(ResponsePrefix) + content
}

// RenderResponseBlock renders multiple lines with tree connectors:
//
//	  ⎿  first line
//	  │  second line
//	  │  third line
func RenderResponseBlock(lines []string, theme ToolUITheme) string {
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, line := range lines {
		if i == 0 {
			sb.WriteString(theme.TreeConn.Render(ResponsePrefix))
		} else {
			sb.WriteString("\n")
			sb.WriteString(theme.TreeConn.Render("  │  "))
		}
		sb.WriteString(line)
	}
	return sb.String()
}

// RenderOutputBlock renders output lines with │ prefix (used inside results).
//
//	  │ output line 1
//	  │ output line 2
func RenderOutputBlock(lines []string, theme ToolUITheme) string {
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(theme.TreeConn.Render("  │ "))
		sb.WriteString(theme.Output.Render(line))
	}
	return sb.String()
}
