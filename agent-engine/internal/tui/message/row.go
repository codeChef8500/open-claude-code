package message

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
)

// RenderOpts controls how messages are rendered.
type RenderOpts struct {
	Width         int
	Dark          bool
	ShowTimestamp bool
	Collapsed     bool
	Styles        *MessageStyles
}

// MessageStyles holds theme-aware styles for message rendering.
// These are built from the themes.Theme color values to match claude-code-main.
type MessageStyles struct {
	Dot        lipgloss.Style // ● prefix color (theme.Claude)
	DotBold    lipgloss.Style // ● prefix for user (bold)
	Connector  lipgloss.Style // ⎿ connector (faint/dim)
	Dim        lipgloss.Style // dim text
	Error      lipgloss.Style // error text (theme.Error)
	ErrorBold  lipgloss.Style // error prefix bold
	System     lipgloss.Style // system text (theme.Suggestion)
	ToolResult lipgloss.Style // tool result dim
	Thinking   lipgloss.Style // thinking text italic dim
	Compact    lipgloss.Style // compact boundary dim
	ToolIcon   lipgloss.Style // tool icon (theme.Claude)
}

// DefaultMessageStyles returns styles using hardcoded dark-theme ANSI colors
// as a fallback when no theme is provided.
func DefaultMessageStyles() *MessageStyles {
	return &MessageStyles{
		Dot:        lipgloss.NewStyle().Foreground(lipgloss.Color("#d77757")),
		DotBold:    lipgloss.NewStyle().Foreground(lipgloss.Color("#d77757")).Bold(true),
		Connector:  lipgloss.NewStyle().Faint(true),
		Dim:        lipgloss.NewStyle().Faint(true),
		Error:      lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b80")),
		ErrorBold:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b80")).Bold(true),
		System:     lipgloss.NewStyle().Foreground(lipgloss.Color("#6495ed")).Italic(true),
		ToolResult: lipgloss.NewStyle().Faint(true),
		Thinking:   lipgloss.NewStyle().Faint(true).Italic(true),
		Compact:    lipgloss.NewStyle().Faint(true),
		ToolIcon:   lipgloss.NewStyle().Foreground(lipgloss.Color("#d77757")),
	}
}

// NewMessageStyles builds MessageStyles from theme color strings.
func NewMessageStyles(claude, errorC, suggestion, inactive string) *MessageStyles {
	c := func(s string) lipgloss.Color { return color.Resolve(s) }
	return &MessageStyles{
		Dot:        lipgloss.NewStyle().Foreground(c(claude)),
		DotBold:    lipgloss.NewStyle().Foreground(c(claude)).Bold(true),
		Connector:  lipgloss.NewStyle().Faint(true),
		Dim:        lipgloss.NewStyle().Foreground(c(inactive)),
		Error:      lipgloss.NewStyle().Foreground(c(errorC)),
		ErrorBold:  lipgloss.NewStyle().Foreground(c(errorC)).Bold(true),
		System:     lipgloss.NewStyle().Foreground(c(suggestion)).Italic(true),
		ToolResult: lipgloss.NewStyle().Foreground(c(inactive)),
		Thinking:   lipgloss.NewStyle().Foreground(c(inactive)).Italic(true),
		Compact:    lipgloss.NewStyle().Foreground(c(inactive)),
		ToolIcon:   lipgloss.NewStyle().Foreground(c(claude)),
	}
}

// blackCircle returns the platform-appropriate filled circle glyph.
func blackCircle() string {
	if runtime.GOOS == "darwin" {
		return "⏺"
	}
	return "●"
}

// styles returns the MessageStyles from opts, or defaults.
func (o RenderOpts) styles() *MessageStyles {
	if o.Styles != nil {
		return o.Styles
	}
	return DefaultMessageStyles()
}

// RenderMessageRow renders a single message for display.
func RenderMessageRow(msg RenderableMessage, opts RenderOpts) string {
	switch msg.Type {
	case TypeUser:
		return RenderUserMessage(msg, opts)
	case TypeAssistant:
		return RenderAssistantMessage(msg, opts)
	case TypeSystem:
		return RenderSystemMessage(msg, opts)
	case TypeError:
		return RenderErrorMessage(msg, opts)
	case TypeToolUse:
		return RenderToolUseMessage(msg, opts)
	case TypeToolResult:
		return RenderToolResultMessage(msg, opts)
	case TypeThinking:
		return RenderThinkingMessage(msg, opts)
	case TypeCompact:
		return RenderCompactBoundary(opts)
	default:
		return msg.PlainText()
	}
}

// RenderUserMessage renders a user message.
// Format: ❯ <content>  (no "You:" label, matching claude-code-main)
func RenderUserMessage(msg RenderableMessage, opts RenderOpts) string {
	s := opts.styles()
	prefix := s.DotBold.Render("❯")
	text := msg.PlainText()
	if opts.ShowTimestamp {
		ts := s.Dim.Render(msg.Timestamp.Format("15:04"))
		return fmt.Sprintf("%s %s\n%s", prefix, ts, text)
	}
	return fmt.Sprintf("%s %s", prefix, text)
}

// RenderAssistantMessage renders an assistant message.
// Format: ● <content>  (using BlackCircle + theme.Claude, no "Assistant:" label)
// Subsequent lines use the ⎿ connector for indentation.
func RenderAssistantMessage(msg RenderableMessage, opts RenderOpts) string {
	s := opts.styles()
	connector := s.Connector.Render("  ⎿  ")

	var sb strings.Builder

	firstLine := true
	for _, block := range msg.Content {
		switch block.Type {
		case BlockText:
			lines := strings.Split(block.Text, "\n")
			for _, line := range lines {
				if firstLine {
					sb.WriteString(s.Dot.Render(blackCircle()))
					sb.WriteString(" ")
					sb.WriteString(line)
					if opts.ShowTimestamp {
						sb.WriteString(" ")
						sb.WriteString(s.Dim.Render(msg.Timestamp.Format("15:04")))
					}
					sb.WriteString("\n")
					firstLine = false
				} else {
					sb.WriteString(connector)
					sb.WriteString(line)
					sb.WriteString("\n")
				}
			}
		case BlockThinking:
			sb.WriteString(connector)
			sb.WriteString(s.Thinking.Render("💭 " + truncateLines(block.Thinking, 3)))
			sb.WriteString("\n")
		case BlockToolUse:
			if block.ToolUse != nil {
				sb.WriteString(connector)
				sb.WriteString(s.ToolIcon.Render("⚙ " + block.ToolUse.Name))
				sb.WriteString("\n")
			}
		}
	}

	// If no content blocks produced output, render a simple dot
	if firstLine {
		sb.WriteString(s.Dot.Render(blackCircle()))
		sb.WriteString(" ")
		sb.WriteString(msg.PlainText())
	}

	return strings.TrimRight(sb.String(), "\n")
}

// RenderSystemMessage renders a system notification.
func RenderSystemMessage(msg RenderableMessage, opts RenderOpts) string {
	s := opts.styles()
	return s.System.Render("▶ " + msg.PlainText())
}

// RenderErrorMessage renders an error message.
func RenderErrorMessage(msg RenderableMessage, opts RenderOpts) string {
	s := opts.styles()
	return s.ErrorBold.Render("⚠ " + msg.PlainText())
}

// RenderToolUseMessage renders a tool call start.
// Format: ● <ToolDisplayName>\n  ⎿  <summary>
func RenderToolUseMessage(msg RenderableMessage, opts RenderOpts) string {
	s := opts.styles()
	name := msg.ToolName
	if name == "" {
		name = "tool"
	}
	display := toolDisplayName(name)

	var sb strings.Builder
	sb.WriteString(s.Dot.Render(blackCircle()))
	sb.WriteString(" ")
	sb.WriteString(s.ToolIcon.Render(display))

	// Show summarized input
	if msg.ToolInput != nil {
		summary := summarizeToolInput(name, msg.ToolInput, opts.Width-10)
		if summary != "" {
			sb.WriteString("\n")
			sb.WriteString(s.Connector.Render("  ⎿  "))
			sb.WriteString(s.Dim.Render(summary))
		}
	}

	return sb.String()
}

// RenderToolResultMessage renders a tool result.
// Format:   ⎿  <output or error>
func RenderToolResultMessage(msg RenderableMessage, opts RenderOpts) string {
	s := opts.styles()
	output := msg.ToolResult
	if output == "" {
		output = msg.PlainText()
	}

	connector := s.Connector.Render("  ⎿  ")

	if msg.IsError {
		return connector + s.Error.Render(truncateLines(output, 5))
	}

	// Collapse long output
	lines := strings.Split(output, "\n")
	if len(lines) > 8 && opts.Collapsed {
		visible := strings.Join(lines[:3], "\n")
		return connector + s.ToolResult.Render(visible) + "\n" +
			s.Dim.Render(fmt.Sprintf("     … (%d lines collapsed)", len(lines)-3))
	}

	if len(lines) > 20 {
		visible := strings.Join(lines[:10], "\n")
		return connector + s.ToolResult.Render(visible) + "\n" +
			s.Dim.Render(fmt.Sprintf("     … (%d more lines)", len(lines)-10))
	}

	return connector + s.ToolResult.Render(output)
}

// RenderThinkingMessage renders a thinking block.
func RenderThinkingMessage(msg RenderableMessage, opts RenderOpts) string {
	s := opts.styles()
	text := msg.ThinkingText
	if text == "" {
		text = msg.PlainText()
	}
	connector := s.Connector.Render("  ⎿  ")
	return connector + s.Thinking.Render("💭 "+truncateLines(text, 3))
}

// RenderCompactBoundary renders a context compaction boundary.
func RenderCompactBoundary(opts RenderOpts) string {
	s := opts.styles()
	w := opts.Width
	if w < 10 {
		w = 60
	}
	line := strings.Repeat("─", w)
	return s.Compact.Render(line + "\n" + "  Context compacted above this line" + "\n" + line)
}

// RenderStreamingToolUse renders an in-progress tool use.
func RenderStreamingToolUse(stu StreamingToolUse, opts RenderOpts) string {
	s := opts.styles()
	elapsed := time.Since(stu.Started).Truncate(time.Second)
	display := toolDisplayName(stu.Name)
	header := s.Dot.Render(blackCircle()) + " " + s.ToolIcon.Render(display)

	if stu.Finished {
		connector := s.Connector.Render("  ⎿  ")
		if stu.IsError {
			return header + "\n" + connector + s.Error.Render(truncateLines(stu.Output, 5))
		}
		return header + "\n" + connector + s.ToolResult.Render(truncateLines(stu.Output, 5))
	}

	return header + s.Dim.Render(fmt.Sprintf(" (%s)", elapsed))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// toolDisplayName returns a human-readable display name for a tool,
// matching claude-code-main's tool display style.
func toolDisplayName(name string) string {
	switch name {
	case "Bash", "bash":
		return "Running command"
	case "Read", "read":
		return "Reading file"
	case "Edit", "edit":
		return "Editing file"
	case "Write", "write":
		return "Writing file"
	case "Glob", "glob":
		return "Searching files"
	case "Grep", "grep":
		return "Searching content"
	case "WebSearch", "web_search":
		return "Searching web"
	case "WebFetch", "web_fetch":
		return "Fetching URL"
	default:
		return name
	}
}

// summarizeToolInput generates a human-readable summary of tool input.
func summarizeToolInput(toolName string, input map[string]interface{}, maxWidth int) string {
	switch toolName {
	case "Bash", "bash":
		if cmd, ok := input["command"].(string); ok {
			return truncateLine(cmd, maxWidth)
		}
	case "Read", "read":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "Edit", "edit":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "Write", "write":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "Glob", "glob":
		if pat, ok := input["pattern"].(string); ok {
			return pat
		}
	case "Grep", "grep":
		if pat, ok := input["pattern"].(string); ok {
			if dir, ok := input["path"].(string); ok {
				return fmt.Sprintf("%q in %s", pat, dir)
			}
			return fmt.Sprintf("%q", pat)
		}
	case "WebSearch", "web_search":
		if q, ok := input["query"].(string); ok {
			return q
		}
	case "WebFetch", "web_fetch":
		if u, ok := input["url"].(string); ok {
			return truncateLine(u, maxWidth)
		}
	}

	// Fallback: show first key=value
	for k, v := range input {
		s := fmt.Sprintf("%s=%v", k, v)
		return truncateLine(s, maxWidth)
	}
	return ""
}

// truncateLine shortens a single line.
func truncateLine(s string, maxLen int) string {
	s = strings.Split(s, "\n")[0]
	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}

// truncateLines shortens multi-line output to maxLines.
func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	result := strings.Join(lines[:maxLines], "\n")
	return result + fmt.Sprintf("\n… (%d more lines)", len(lines)-maxLines)
}
