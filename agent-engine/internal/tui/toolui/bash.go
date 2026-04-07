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
//
// If the command has a # comment label and verbose is false, the label is shown
// instead of the command, matching claude-code-main's commentLabel behavior.
func (b *BashToolUI) RenderStart(dotView, command string, verbose bool) string {
	params := formatBashParams(command, verbose)
	return RenderToolHeader(dotView, "Bash", params, b.theme)
}

// RenderSedAsEdit renders a sed -i command as a file edit header:
//
//	● Bash (file.txt)
func (b *BashToolUI) RenderSedAsEdit(dotView, filePath string) string {
	return RenderToolHeader(dotView, "Bash", filePath, b.theme)
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

// BashProgress holds live progress data for a running bash command.
type BashProgress struct {
	Output     string
	ElapsedSec float64
	TotalLines int
	TotalBytes int64
	TimeoutMs  int
	TaskID     string
}

// RenderProgress renders a progress line for a running bash command:
//
//	⎿  Running… 12 lines · 3.2KB · 5s
func (b *BashToolUI) RenderProgress(p BashProgress) string {
	var parts []string
	parts = append(parts, "Running…")
	if p.TotalLines > 0 {
		parts = append(parts, fmt.Sprintf("%d lines", p.TotalLines))
	}
	if p.TotalBytes > 0 {
		parts = append(parts, formatBashBytes(p.TotalBytes))
	}
	if p.ElapsedSec >= 1 {
		parts = append(parts, fmt.Sprintf("%.0fs", p.ElapsedSec))
	}
	if p.TimeoutMs > 0 && p.ElapsedSec > 0 {
		pct := (p.ElapsedSec * 1000) / float64(p.TimeoutMs) * 100
		if pct > 50 {
			parts = append(parts, fmt.Sprintf("%.0f%% timeout", pct))
		}
	}
	msg := strings.Join(parts, " · ")
	return RenderResponseLine(b.theme.Dim.Render(msg), b.theme)
}

// BackgroundHintText returns the shortcut hint text for backgrounding.
func (b *BashToolUI) BackgroundHintText(shortcut string) string {
	if shortcut == "" {
		shortcut = "ctrl+b"
	}
	return b.theme.Dim.Render(fmt.Sprintf("     (%s to run in background)", shortcut))
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
// In non-verbose mode, a first-line # comment label takes precedence over the command.
func formatBashParams(command string, verbose bool) string {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return ""
	}

	if verbose {
		return "$ " + cmd
	}

	// Comment label: if first line is "# label", show just the label text.
	label := extractCommentLabel(cmd)
	if label != "" {
		if len(label) > MaxCommandDisplayChars {
			return label[:MaxCommandDisplayChars] + "…"
		}
		return label
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

// extractCommentLabel returns the comment label from the first line of a command.
// Returns empty string if no comment label.
func extractCommentLabel(command string) string {
	nl := strings.IndexByte(command, '\n')
	first := command
	if nl >= 0 {
		first = command[:nl]
	}
	first = strings.TrimSpace(first)
	if !strings.HasPrefix(first, "#") || strings.HasPrefix(first, "#!") {
		return ""
	}
	return strings.TrimSpace(strings.TrimLeft(first, "#"))
}

// formatBashBytes formats byte count for progress display.
func formatBashBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// ── sed -i detection ────────────────────────────────────────────────────────

// IsSedInPlace detects if a bash command is a sed -i (in-place edit) command.
// Matches patterns like: sed -i 's/old/new/' file.txt
//
//	sed -i.bak 's/old/new/' file.txt
//	sed --in-place 's/old/new/' file.txt
func IsSedInPlace(command string) bool {
	cmd := strings.TrimSpace(command)
	if !strings.HasPrefix(cmd, "sed ") {
		return false
	}
	parts := strings.Fields(cmd)
	for _, p := range parts[1:] {
		if p == "-i" || strings.HasPrefix(p, "-i") || p == "--in-place" || strings.HasPrefix(p, "--in-place") {
			return true
		}
		if p == "--" {
			break
		}
	}
	return false
}

// ParseSedTarget extracts the target file path from a sed -i command.
// Returns the file path and true if found, or ("", false) if not parseable.
func ParseSedTarget(command string) (string, bool) {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) < 3 || parts[0] != "sed" {
		return "", false
	}
	// Walk arguments: skip flags and expressions, last non-flag arg is the file.
	var lastArg string
	skipNext := false
	for _, p := range parts[1:] {
		if skipNext {
			skipNext = false
			continue
		}
		if p == "-e" || p == "-f" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(p, "-") {
			continue
		}
		lastArg = p
	}
	if lastArg != "" && !strings.HasPrefix(lastArg, "'") && !strings.HasPrefix(lastArg, "\"") && !strings.HasPrefix(lastArg, "s/") && !strings.HasPrefix(lastArg, "s|") {
		return lastArg, true
	}
	return "", false
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
