package toolui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// BashToolUI renders bash/shell tool use with command highlighting and output.
type BashToolUI struct {
	theme ToolUITheme
}

// NewBashToolUI creates a bash tool renderer.
func NewBashToolUI(theme ToolUITheme) *BashToolUI {
	return &BashToolUI{theme: theme}
}

// RenderStart renders a bash tool invocation.
func (b *BashToolUI) RenderStart(command string, width int) string {
	var sb strings.Builder

	sb.WriteString(b.theme.ToolIcon.Render("⚙ Bash"))
	sb.WriteString("\n")

	// Render command with shell syntax hint
	cmd := strings.TrimSpace(command)
	lines := strings.Split(cmd, "\n")

	if len(lines) == 1 {
		sb.WriteString(b.theme.TreeConn.Render("  ⎿ "))
		sb.WriteString(b.theme.Code.Render("$ " + cmd))
	} else {
		for i, line := range lines {
			prefix := "  │ "
			if i == len(lines)-1 {
				prefix = "  ⎿ "
			}
			sb.WriteString(b.theme.TreeConn.Render(prefix))
			if i == 0 {
				sb.WriteString(b.theme.Code.Render("$ " + line))
			} else {
				sb.WriteString(b.theme.Code.Render("  " + line))
			}
			if i < len(lines)-1 {
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// RenderResult renders bash tool output.
func (b *BashToolUI) RenderResult(output string, exitCode int, elapsed time.Duration, width int) string {
	var sb strings.Builder

	maxLines := 15
	lines := strings.Split(output, "\n")

	// Status line
	if exitCode != 0 {
		sb.WriteString(b.theme.Error.Render(fmt.Sprintf("  ✗ Exit code %d", exitCode)))
	} else {
		sb.WriteString(b.theme.Success.Render(fmt.Sprintf("  ✓ Done (%s)", elapsed.Truncate(time.Millisecond))))
	}
	sb.WriteString("\n")

	// Output lines
	if len(lines) > maxLines {
		for _, line := range lines[:maxLines/2] {
			sb.WriteString(b.theme.Output.Render("  │ " + truncateLine(line, width-6)))
			sb.WriteString("\n")
		}
		sb.WriteString(b.theme.Dim.Render(fmt.Sprintf("  │ … (%d lines omitted)", len(lines)-maxLines)))
		sb.WriteString("\n")
		for _, line := range lines[len(lines)-maxLines/2:] {
			sb.WriteString(b.theme.Output.Render("  │ " + truncateLine(line, width-6)))
			sb.WriteString("\n")
		}
	} else {
		for _, line := range lines {
			sb.WriteString(b.theme.Output.Render("  │ " + truncateLine(line, width-6)))
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// RenderStreaming renders a bash command that's currently executing.
func (b *BashToolUI) RenderStreaming(command string, elapsed time.Duration) string {
	return b.theme.ToolIcon.Render("⚙ Bash") + " " +
		b.theme.Dim.Render(fmt.Sprintf("(%s)", elapsed.Truncate(time.Second))) + "\n" +
		b.theme.TreeConn.Render("  ⎿ ") +
		b.theme.Code.Render("$ "+firstLine(command))
}

// ── helpers ──────────────────────────────────────────────────────────────────

func truncateLine(s string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 80
	}
	vis := lipgloss.Width(s)
	if vis <= maxLen {
		return s
	}
	// Rough truncation for ANSI strings
	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx] + "…"
	}
	return s
}
