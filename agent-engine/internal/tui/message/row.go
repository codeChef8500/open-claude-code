package message

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RenderOpts controls how messages are rendered.
type RenderOpts struct {
	Width       int
	Dark        bool
	ShowTimestamp bool
	Collapsed   bool
}

// Styles used for message rendering.
var (
	userPrefix     = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	assistantPrefix = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	systemPrefix   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Italic(true)
	errorPrefix    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	toolUsePrefix  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Italic(true)
	toolResultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	thinkingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	compactStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorResultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	treeConnector  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

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
// Format: ❯ You: <content>
func RenderUserMessage(msg RenderableMessage, opts RenderOpts) string {
	prefix := userPrefix.Render("❯ You")
	text := msg.PlainText()
	if opts.ShowTimestamp {
		ts := dimStyle.Render(msg.Timestamp.Format("15:04"))
		return fmt.Sprintf("%s %s\n%s", prefix, ts, text)
	}
	return fmt.Sprintf("%s\n%s", prefix, text)
}

// RenderAssistantMessage renders an assistant message.
// Format: ⏺ Assistant:\n<markdown content>
func RenderAssistantMessage(msg RenderableMessage, opts RenderOpts) string {
	prefix := assistantPrefix.Render("⏺ Assistant")
	var sb strings.Builder
	sb.WriteString(prefix)
	if opts.ShowTimestamp {
		sb.WriteString(" ")
		sb.WriteString(dimStyle.Render(msg.Timestamp.Format("15:04")))
	}
	sb.WriteString("\n")

	for _, block := range msg.Content {
		switch block.Type {
		case BlockText:
			sb.WriteString(block.Text)
			sb.WriteString("\n")
		case BlockThinking:
			sb.WriteString(thinkingStyle.Render("  💭 "+truncateLines(block.Thinking, 3)))
			sb.WriteString("\n")
		case BlockToolUse:
			if block.ToolUse != nil {
				sb.WriteString(toolUsePrefix.Render("  ⚙ "+block.ToolUse.Name))
				sb.WriteString("\n")
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// RenderSystemMessage renders a system notification.
func RenderSystemMessage(msg RenderableMessage, opts RenderOpts) string {
	return systemPrefix.Render("▶ " + msg.PlainText())
}

// RenderErrorMessage renders an error message.
func RenderErrorMessage(msg RenderableMessage, opts RenderOpts) string {
	return errorPrefix.Render("⚠ " + msg.PlainText())
}

// RenderToolUseMessage renders a tool call start.
// Format: ⚙ <ToolName> <summary>
func RenderToolUseMessage(msg RenderableMessage, opts RenderOpts) string {
	name := msg.ToolName
	if name == "" {
		name = "tool"
	}

	var sb strings.Builder
	sb.WriteString(toolUsePrefix.Render("⚙ " + name))

	// Show summarized input
	if msg.ToolInput != nil {
		summary := summarizeToolInput(name, msg.ToolInput, opts.Width-10)
		if summary != "" {
			sb.WriteString("\n")
			sb.WriteString(treeConnector.Render("  ⎿ "))
			sb.WriteString(dimStyle.Render(summary))
		}
	}

	return sb.String()
}

// RenderToolResultMessage renders a tool result.
// Format:   ⎿ <output or error>
func RenderToolResultMessage(msg RenderableMessage, opts RenderOpts) string {
	output := msg.ToolResult
	if output == "" {
		output = msg.PlainText()
	}

	connector := treeConnector.Render("  ⎿ ")

	if msg.IsError {
		return connector + errorResultStyle.Render(truncateLines(output, 5))
	}

	// Collapse long output
	lines := strings.Split(output, "\n")
	if len(lines) > 8 && opts.Collapsed {
		visible := strings.Join(lines[:3], "\n")
		return connector + toolResultStyle.Render(visible) + "\n" +
			dimStyle.Render(fmt.Sprintf("    … (%d lines collapsed)", len(lines)-3))
	}

	if len(lines) > 20 {
		visible := strings.Join(lines[:10], "\n")
		return connector + toolResultStyle.Render(visible) + "\n" +
			dimStyle.Render(fmt.Sprintf("    … (%d more lines)", len(lines)-10))
	}

	return connector + toolResultStyle.Render(output)
}

// RenderThinkingMessage renders a thinking block.
func RenderThinkingMessage(msg RenderableMessage, opts RenderOpts) string {
	text := msg.ThinkingText
	if text == "" {
		text = msg.PlainText()
	}
	return thinkingStyle.Render("  💭 " + truncateLines(text, 3))
}

// RenderCompactBoundary renders a context compaction boundary.
func RenderCompactBoundary(opts RenderOpts) string {
	w := opts.Width
	if w < 10 {
		w = 60
	}
	line := strings.Repeat("─", w)
	return compactStyle.Render(line + "\n" + "  Context compacted above this line" + "\n" + line)
}

// RenderStreamingToolUse renders an in-progress tool use.
func RenderStreamingToolUse(stu StreamingToolUse, opts RenderOpts) string {
	elapsed := time.Since(stu.Started).Truncate(time.Second)
	header := toolUsePrefix.Render(fmt.Sprintf("⚙ %s", stu.Name))

	if stu.Finished {
		if stu.IsError {
			return header + "\n" + treeConnector.Render("  ✗ ") +
				errorResultStyle.Render(truncateLines(stu.Output, 5))
		}
		return header + "\n" + treeConnector.Render("  ⎿ ") +
			toolResultStyle.Render(truncateLines(stu.Output, 5))
	}

	return header + dimStyle.Render(fmt.Sprintf(" (%s)", elapsed))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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
