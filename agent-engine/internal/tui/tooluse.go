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

	// Streaming progress data (updated by EventToolProgress)
	BashLines   int    // Bash: total output lines so far
	BashBytes   int64  // Bash: total output bytes so far
	SearchQuery string // WebSearch: query text
	SearchHits  int    // WebSearch: results received so far
	FetchPhase  string // WebFetch: "connecting", "downloading", "processing"
	FetchBytes  int    // WebFetch: bytes read so far
	FetchStatus int    // WebFetch: HTTP status code
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

// UpdateProgress updates streaming progress for an active tool call.
func (t *ToolUseTracker) UpdateProgress(id string, progressType string, data map[string]interface{}) {
	state, ok := t.active[id]
	if !ok {
		return
	}
	switch progressType {
	case "bash":
		if v, ok := data["output_lines"]; ok {
			if n, ok := v.(float64); ok {
				state.BashLines = int(n)
			}
		}
		if v, ok := data["output_bytes"]; ok {
			if n, ok := v.(float64); ok {
				state.BashBytes = int64(n)
			}
		}
	case "web_search":
		if v, ok := data["query"]; ok {
			if s, ok := v.(string); ok {
				state.SearchQuery = s
			}
		}
		if v, ok := data["results_received"]; ok {
			if n, ok := v.(float64); ok {
				state.SearchHits = int(n)
			}
		}
	case "web_fetch":
		if v, ok := data["phase"]; ok {
			if s, ok := v.(string); ok {
				state.FetchPhase = s
			}
		}
		if v, ok := data["bytes_read"]; ok {
			if n, ok := v.(float64); ok {
				state.FetchBytes = int(n)
			}
		}
		if v, ok := data["status_code"]; ok {
			if n, ok := v.(float64); ok {
				state.FetchStatus = int(n)
			}
		}
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

// RenderActive renders the active tool calls using enhanced per-tool renderers:
//
//	● Bash ($ git status)
//	  ⎿  Running… 12 lines · 3.2KB · 5s
func (t *ToolUseTracker) RenderActive() string {
	if len(t.active) == 0 {
		return ""
	}

	theme := t.buildToolUITheme()
	dotStyle := lipgloss.NewStyle().Foreground(t.styles.ToolUse.GetForeground())
	dot := dotStyle.Render(figures.BlackCircle()) + " "

	var lines []string
	for _, s := range t.active {
		elapsed := s.Duration()

		switch s.ToolName {
		case "Bash", "bash":
			ui := toolui.NewBashToolUI(theme)
			// Use streaming progress data if available.
			if s.BashLines > 0 || s.BashBytes > 0 {
				progress := toolui.BashProgress{
					Output:     s.Output,
					ElapsedSec: elapsed.Seconds(),
					TotalLines: s.BashLines,
					TotalBytes: s.BashBytes,
				}
				header := ui.RenderStart(dot, s.Input, false)
				progressLine := ui.RenderProgress(progress)
				lines = append(lines, header+"\n"+progressLine)
			} else if s.Output != "" {
				outLines := strings.Split(s.Output, "\n")
				lines = append(lines, ui.RenderStreamingWithOutput(dot, s.Input, outLines, elapsed, 100))
			} else {
				lines = append(lines, ui.RenderStreaming(dot, s.Input, elapsed))
			}
		case "Edit", "edit":
			ui := toolui.NewEditToolUI(theme)
			header := ui.RenderStart(dot, "Update", s.Input, false)
			running := toolui.RenderResponseLine(t.styles.Dimmed.Render("Applying…"), theme)
			lines = append(lines, header+"\n"+running)
		case "WebSearch", "web_search":
			ui := toolui.NewWebSearchToolUI(theme)
			query := s.SearchQuery
			if query == "" {
				query = s.Input
			}
			header := ui.RenderStart(dot, query)
			var progress string
			if s.SearchHits > 0 {
				progress = toolui.RenderResponseLine(
					t.styles.Dimmed.Render(fmt.Sprintf("Found %d results (%s)", s.SearchHits, elapsed.Round(time.Millisecond))),
					theme)
			} else {
				progress = ui.RenderProgress(query)
			}
			lines = append(lines, header+"\n"+progress)
		case "WebFetch", "web_fetch":
			ui := toolui.NewWebFetchToolUI(theme)
			header := ui.RenderStart(dot, s.Input)
			var progress string
			if s.FetchPhase != "" {
				progress = ui.RenderProgressPhase(s.FetchPhase, s.FetchBytes, s.FetchStatus)
			} else {
				progress = ui.RenderProgress()
			}
			lines = append(lines, header+"\n"+progress)
		default:
			header := toolui.RenderToolHeader(dot, s.ToolName, s.Input, theme)
			running := t.styles.Dimmed.Render(fmt.Sprintf("Running… (%s)", elapsed.Round(time.Millisecond)))
			result := toolui.RenderResponseLine(running, theme)
			lines = append(lines, header+"\n"+result)
		}
	}
	return strings.Join(lines, "\n")
}

// RenderCompleted renders the last N completed tool calls using enhanced per-tool renderers:
//
//	● Bash ($ git status)
//	  ⎿  Ran (123ms)
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
		var dotColor lipgloss.TerminalColor
		if s.IsError {
			dotColor = t.styles.Error.GetForeground()
		} else {
			dotColor = t.styles.Success.GetForeground()
		}
		dotStyle := lipgloss.NewStyle().Foreground(dotColor)
		dot := dotStyle.Render(figures.BlackCircle()) + " "
		elapsed := s.Duration()

		switch s.ToolName {
		case "Bash", "bash":
			ui := toolui.NewBashToolUI(theme)
			header := ui.RenderStart(dot, s.Input, false)
			result := ui.RenderResult(s.Output, 0, elapsed, 100)
			lines = append(lines, header+"\n"+result)
		case "Edit", "edit":
			ui := toolui.NewEditToolUI(theme)
			header := ui.RenderStart(dot, "Update", s.Input, false)
			result := ui.RenderResultSimple(!s.IsError, elapsed, 0)
			lines = append(lines, header+"\n"+result)
		case "Write", "write":
			ui := toolui.NewWriteToolUI(theme)
			header := ui.RenderStart(dot, s.Input, false)
			result := ui.RenderResult(!s.IsError, elapsed)
			lines = append(lines, header+"\n"+result)
		case "Read", "read":
			ui := toolui.NewReadToolUI(theme)
			header := ui.RenderStart(dot, s.Input, "", false)
			lineCount := strings.Count(s.Output, "\n")
			result := ui.RenderResult(s.Output, lineCount, elapsed, 100, false)
			lines = append(lines, header+"\n"+result)
		default:
			header := toolui.RenderToolHeader(dot, s.ToolName, s.Input, theme)
			var resultMsg string
			if s.IsError {
				resultMsg = t.styles.Error.Render(fmt.Sprintf("Error (%s)", elapsed.Round(time.Millisecond)))
			} else {
				resultMsg = t.styles.Dimmed.Render(fmt.Sprintf("Done (%s)", elapsed.Round(time.Millisecond)))
			}
			result := toolui.RenderResponseLine(resultMsg, theme)
			lines = append(lines, header+"\n"+result)
		}
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
