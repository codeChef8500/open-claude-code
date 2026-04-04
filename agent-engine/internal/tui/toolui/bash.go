package toolui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// MaxCommandDisplayLines limits how many lines of command are shown in compact mode.
const MaxCommandDisplayLines = 2

// MaxCommandDisplayChars limits the character width of a displayed command.
const MaxCommandDisplayChars = 160

// BashToolUI renders bash/shell tool use with command highlighting and output.
// Layout matches claude-code-main:
//
//	● Bash ($ git status)
//	  ⎿  Running…
//	  ⎿  <output lines>
type BashToolUI struct {
	theme ToolUITheme
}

// NewBashToolUI creates a bash tool renderer.
func NewBashToolUI(theme ToolUITheme) *BashToolUI {
	return &BashToolUI{theme: theme}
}

// RenderStart renders a bash tool header line:
//
//	● Bash ($ git diff --stat)
func (b *BashToolUI) RenderStart(dotView, command string, verbose bool) string {
	params := formatBashParams(command, verbose)
	return RenderToolHeader(dotView, "Bash", params, b.theme)
}

// RenderResult renders bash tool output with ⎿ connector:
//
//	⎿  <status line>
//	│  <output lines>
func (b *BashToolUI) RenderResult(output string, exitCode int, elapsed time.Duration, width int) string {
	var sb strings.Builder

	maxLines := 15
	lines := strings.Split(output, "\n")

	// Status line with ⎿ connector
	if exitCode != 0 {
		status := b.theme.Error.Render(fmt.Sprintf("Exit code %d (%s)", exitCode, elapsed.Truncate(time.Millisecond)))
		sb.WriteString(RenderResponseLine(status, b.theme))
	} else {
		status := b.theme.Dim.Render(fmt.Sprintf("Ran (%s)", elapsed.Truncate(time.Millisecond)))
		sb.WriteString(RenderResponseLine(status, b.theme))
	}

	// Output lines
	if len(lines) > 0 && !(len(lines) == 1 && lines[0] == "") {
		sb.WriteString("\n")
		if len(lines) > maxLines {
			for _, line := range lines[:maxLines/2] {
				sb.WriteString(b.theme.TreeConn.Render("  │ "))
				sb.WriteString(b.theme.Output.Render(truncateLine(line, width-6)))
				sb.WriteString("\n")
			}
			sb.WriteString(b.theme.Dim.Render(fmt.Sprintf("  │ … (%d lines omitted)", len(lines)-maxLines)))
			sb.WriteString("\n")
			for _, line := range lines[len(lines)-maxLines/2:] {
				sb.WriteString(b.theme.TreeConn.Render("  │ "))
				sb.WriteString(b.theme.Output.Render(truncateLine(line, width-6)))
				sb.WriteString("\n")
			}
		} else {
			for _, line := range lines {
				sb.WriteString(b.theme.TreeConn.Render("  │ "))
				sb.WriteString(b.theme.Output.Render(truncateLine(line, width-6)))
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// RenderStreaming renders a bash command that's currently executing:
//
//	● Bash ($ command…)
//	  ⎿  Running…
func (b *BashToolUI) RenderStreaming(dotView, command string, elapsed time.Duration) string {
	header := b.RenderStart(dotView, command, false)
	running := b.theme.Dim.Render("Running…")
	return header + "\n" + RenderResponseLine(running, b.theme)
}

// RenderStreamingWithOutput renders a bash command with live output tail:
//
//	● Bash ($ command…)
//	  ⎿  Running…
//	  │  <last few lines of output>
func (b *BashToolUI) RenderStreamingWithOutput(dotView, command string, lastLines []string, elapsed time.Duration, width int) string {
	header := b.RenderStart(dotView, command, false)
	running := b.theme.Dim.Render(fmt.Sprintf("Running… (%s)", elapsed.Truncate(time.Second)))
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(RenderResponseLine(running, b.theme))

	maxTail := 5
	show := lastLines
	if len(show) > maxTail {
		show = show[len(show)-maxTail:]
	}
	for _, line := range show {
		sb.WriteString("\n")
		sb.WriteString(b.theme.TreeConn.Render("  │ "))
		sb.WriteString(b.theme.Output.Render(truncateLine(line, width-6)))
	}

	return sb.String()
}

// formatBashParams formats the command display for the header parenthesized section.
func formatBashParams(command string, verbose bool) string {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return ""
	}

	if verbose {
		return "$ " + cmd
	}

	lines := strings.Split(cmd, "\n")
	if len(lines) > MaxCommandDisplayLines {
		cmd = strings.Join(lines[:MaxCommandDisplayLines], "\n")
	}
	if len(cmd) > MaxCommandDisplayChars {
		cmd = cmd[:MaxCommandDisplayChars] + "…"
	} else if len(lines) > MaxCommandDisplayLines {
		cmd += "…"
	}

	// Collapse newlines to spaces for compact display
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	return "$ " + cmd
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
