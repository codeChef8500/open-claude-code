package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/figures"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
	"github.com/wall-ai/agent-engine/internal/tui/toolui"
)

// ToolUseState tracks the display state of an in-flight tool call.
type ToolUseState struct {
	ToolName  string
	ToolID    string
	Input     string
	Output    string
	IsError   bool
	StartTime time.Time
	EndTime   time.Time
	Done      bool
}

// Duration returns the elapsed time for this tool call.
func (t *ToolUseState) Duration() time.Duration {
	if t.Done {
		return t.EndTime.Sub(t.StartTime)
	}
	return time.Since(t.StartTime)
}

// ToolUseTracker manages the display of active and completed tool calls.
type ToolUseTracker struct {
	active    map[string]*ToolUseState
	completed []*ToolUseState
	styles    themes.Styles
}

// NewToolUseTracker creates a new tracker.
func NewToolUseTracker(styles themes.Styles) *ToolUseTracker {
	return &ToolUseTracker{
		active: make(map[string]*ToolUseState),
		styles: styles,
	}
}

// StartTool records a new tool call.
func (t *ToolUseTracker) StartTool(id, name, input string) {
	t.active[id] = &ToolUseState{
		ToolName:  name,
		ToolID:    id,
		Input:     truncateInput(input, 120),
		StartTime: time.Now(),
	}
}

// FinishTool marks a tool call as completed.
func (t *ToolUseTracker) FinishTool(id, output string, isError bool) {
	if state, ok := t.active[id]; ok {
		state.Output = truncateInput(output, 200)
		state.IsError = isError
		state.EndTime = time.Now()
		state.Done = true
		t.completed = append(t.completed, state)
		delete(t.active, id)
	}
}

// HasActive reports whether there are in-flight tool calls.
func (t *ToolUseTracker) HasActive() bool {
	return len(t.active) > 0
}

// ActiveCount returns the number of active tool calls.
func (t *ToolUseTracker) ActiveCount() int {
	return len(t.active)
}

// RenderActive renders the active tool calls using claude-code-main format:
//
//	● ToolName (params)
//	  ⎿  Running…
func (t *ToolUseTracker) RenderActive() string {
	if len(t.active) == 0 {
		return ""
	}

	theme := t.buildToolUITheme()

	var lines []string
	for _, s := range t.active {
		// Blinking dot for active state
		dotStyle := lipgloss.NewStyle().Foreground(t.styles.ToolUse.GetForeground())
		dot := dotStyle.Render(figures.BlackCircle()) + " "

		header := toolui.RenderToolHeader(dot, s.ToolName, s.Input, theme)

		elapsed := s.Duration().Round(time.Millisecond)
		running := t.styles.Dimmed.Render(fmt.Sprintf("Running… (%s)", elapsed))
		result := toolui.RenderResponseLine(running, theme)

		lines = append(lines, header+"\n"+result)
	}
	return strings.Join(lines, "\n")
}

// RenderCompleted renders the last N completed tool calls using claude-code format:
//
//	● ToolName (params)
//	  ⎿  Done (123ms)
func (t *ToolUseTracker) RenderCompleted(n int) string {
	if len(t.completed) == 0 {
		return ""
	}
	start := 0
	if len(t.completed) > n {
		start = len(t.completed) - n
	}

	theme := t.buildToolUITheme()

	var lines []string
	for _, s := range t.completed[start:] {
		// Green dot for success, red for error
		var dotColor lipgloss.TerminalColor
		if s.IsError {
			dotColor = t.styles.Error.GetForeground()
		} else {
			dotColor = t.styles.Success.GetForeground()
		}
		dotStyle := lipgloss.NewStyle().Foreground(dotColor)
		dot := dotStyle.Render(figures.BlackCircle()) + " "

		header := toolui.RenderToolHeader(dot, s.ToolName, s.Input, theme)

		elapsed := s.Duration().Round(time.Millisecond)
		var resultMsg string
		if s.IsError {
			resultMsg = t.styles.Error.Render(fmt.Sprintf("Error (%s)", elapsed))
		} else {
			resultMsg = t.styles.Dimmed.Render(fmt.Sprintf("Done (%s)", elapsed))
		}
		result := toolui.RenderResponseLine(resultMsg, theme)

		lines = append(lines, header+"\n"+result)
	}
	return strings.Join(lines, "\n")
}

// buildToolUITheme constructs a ToolUITheme from the tracker's styles.
func (t *ToolUseTracker) buildToolUITheme() toolui.ToolUITheme {
	return toolui.ToolUITheme{
		ToolIcon: t.styles.ToolUse,
		TreeConn: t.styles.Connector,
		Code:     t.styles.Highlight,
		Output:   t.styles.Dimmed,
		Dim:      t.styles.Dimmed,
		Error:    t.styles.Error,
		Success:  t.styles.Success,
		FilePath: t.styles.Highlight,
		DiffAdd:  t.styles.DiffAdd,
		DiffDel:  t.styles.DiffDel,
	}
}

// Clear resets the tracker.
func (t *ToolUseTracker) Clear() {
	t.active = make(map[string]*ToolUseState)
	t.completed = nil
}

// ── Status bar helpers ──────────────────────────────────────────────────────

// StatusInfo holds data for the rich status bar.
type StatusInfo struct {
	Model       string
	CostUSD     float64
	InputTokens int
	TurnCount   int
	Mode        string // permission mode
}

// RenderStatusBar builds a rich status bar string.
func RenderStatusBar(info StatusInfo, width int, theme Theme) string {
	left := info.Model
	if info.Mode != "" {
		left += " │ " + info.Mode
	}

	right := ""
	if info.CostUSD > 0 {
		right = fmt.Sprintf("$%.4f", info.CostUSD)
	}
	if info.InputTokens > 0 {
		if right != "" {
			right += " │ "
		}
		right += fmt.Sprintf("%dk tokens", info.InputTokens/1000)
	}
	if info.TurnCount > 0 {
		if right != "" {
			right += " │ "
		}
		right += fmt.Sprintf("turn %d", info.TurnCount)
	}

	// Pad middle.
	pad := width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if pad < 1 {
		pad = 1
	}

	return theme.StatusBar.Width(width).Render(
		" " + left + strings.Repeat(" ", pad) + right + " ",
	)
}

func truncateInput(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
