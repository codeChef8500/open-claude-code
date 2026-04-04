package toolui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/figures"
)

// DotState represents the visual state of a tool-use dot indicator,
// matching claude-code-main's ToolUseLoader component.
type DotState int

const (
	// DotQueued — grey static ● (tool call is queued)
	DotQueued DotState = iota
	// DotActive — blinking ● (tool is executing)
	DotActive
	// DotSuccess — green ● (tool completed successfully)
	DotSuccess
	// DotError — red ● (tool returned an error)
	DotError
	// DotWaitingPermission — dim static ● (waiting for user permission)
	DotWaitingPermission
)

// blinkInterval is the blink cycle period (500ms matches claude-code).
const blinkInterval = 500 * time.Millisecond

// dotBlinkMsg is the internal tick message for dot blink animation.
type dotBlinkMsg time.Time

// ToolDot renders a status dot indicator (●) that can blink when active.
type ToolDot struct {
	state   DotState
	visible bool // toggle for blink animation
	theme   ToolUITheme

	// Colors resolved from theme
	activeColor     lipgloss.TerminalColor
	successColor    lipgloss.TerminalColor
	errorColor      lipgloss.TerminalColor
	queuedColor     lipgloss.TerminalColor
	permissionColor lipgloss.TerminalColor
}

// NewToolDot creates a new ToolDot with the given theme.
func NewToolDot(theme ToolUITheme) *ToolDot {
	return &ToolDot{
		state:           DotQueued,
		visible:         true,
		theme:           theme,
		activeColor:     theme.ToolIcon.GetForeground(),
		successColor:    theme.Success.GetForeground(),
		errorColor:      theme.Error.GetForeground(),
		queuedColor:     theme.Dim.GetForeground(),
		permissionColor: theme.Dim.GetForeground(),
	}
}

// NewToolDotWithColors creates a ToolDot with explicit color overrides.
func NewToolDotWithColors(active, success, errorC, queued lipgloss.TerminalColor) *ToolDot {
	return &ToolDot{
		state:           DotQueued,
		visible:         true,
		activeColor:     active,
		successColor:    success,
		errorColor:      errorC,
		queuedColor:     queued,
		permissionColor: queued,
	}
}

// SetState updates the dot's visual state.
func (d *ToolDot) SetState(s DotState) {
	d.state = s
	// Reset blink visibility when entering active state
	if s == DotActive {
		d.visible = true
	}
}

// State returns the current dot state.
func (d *ToolDot) State() DotState {
	return d.state
}

// View renders the dot glyph with appropriate color, or a space when blinking off.
func (d *ToolDot) View() string {
	glyph := figures.BlackCircle()

	switch d.state {
	case DotActive:
		if !d.visible {
			// Blink off — render a space of equal width
			return " " + " "
		}
		style := lipgloss.NewStyle().Foreground(d.activeColor)
		return style.Render(glyph) + " "

	case DotSuccess:
		style := lipgloss.NewStyle().Foreground(d.successColor)
		return style.Render(glyph) + " "

	case DotError:
		style := lipgloss.NewStyle().Foreground(d.errorColor)
		return style.Render(glyph) + " "

	case DotWaitingPermission:
		style := lipgloss.NewStyle().Foreground(d.permissionColor).Faint(true)
		return style.Render(glyph) + " "

	default: // DotQueued
		style := lipgloss.NewStyle().Foreground(d.queuedColor)
		return style.Render(glyph) + " "
	}
}

// Update processes blink tick messages.
func (d *ToolDot) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(dotBlinkMsg); ok {
		if d.state == DotActive {
			d.visible = !d.visible
			return d.tickCmd()
		}
	}
	return nil
}

// Init starts the blink ticker if the dot is in active state.
func (d *ToolDot) Init() tea.Cmd {
	if d.state == DotActive {
		return d.tickCmd()
	}
	return nil
}

// tickCmd returns a tea.Cmd that fires a dotBlinkMsg after blinkInterval.
func (d *ToolDot) tickCmd() tea.Cmd {
	return tea.Tick(blinkInterval, func(t time.Time) tea.Msg {
		return dotBlinkMsg(t)
	})
}
