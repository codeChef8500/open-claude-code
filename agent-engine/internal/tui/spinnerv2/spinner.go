package spinnerv2

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wall-ai/agent-engine/internal/tui/color"
	"github.com/wall-ai/agent-engine/internal/tui/figures"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
)

// tickInterval is the animation frame interval (120ms matches claude-code-main).
const tickInterval = 120 * time.Millisecond

// stalledThresholdMs is the delay before stalled color transition begins.
const stalledThresholdMs = 5000

// stalledDurationMs is how long the stalled color transition takes (0→1).
const stalledDurationMs = 5000

// spinnerTickMsg is the internal tick message for animation frames.
type spinnerTickMsg time.Time

// SpinnerModel is a custom spinner matching claude-code-main's animation:
//   - Frame sequence: ·✢✳✶✻✽ (forward + reverse oscillation)
//   - 120ms frame interval
//   - Stalled detection: after 5s, color transitions from Claude→Error (red)
//   - Shimmer text effect on the label
//   - Token counter and elapsed time display
type SpinnerModel struct {
	frames  []string
	visible bool
	label   string
	theme   themes.Theme

	startTime  time.Time
	currentMs  int64 // elapsed ms since animation start
	frameIdx   int
	tokenCount int

	// reducedMotion disables animation (static ● glyph).
	reducedMotion bool
}

// New creates a SpinnerModel with the given theme.
func New(theme themes.Theme) *SpinnerModel {
	return &SpinnerModel{
		frames: figures.SpinnerFrames(),
		theme:  theme,
	}
}

// Show makes the spinner visible with a new random verb label.
func (s *SpinnerModel) Show(label string) {
	s.visible = true
	s.label = label
	s.startTime = time.Now()
	s.currentMs = 0
	s.frameIdx = 0
	s.tokenCount = 0
}

// ShowRandom shows the spinner with a random verb from the spinner verbs list.
func (s *SpinnerModel) ShowRandom() {
	s.Show(RandomVerb() + "…")
}

// Hide stops the spinner.
func (s *SpinnerModel) Hide() {
	s.visible = false
	s.label = ""
	s.tokenCount = 0
}

// IsVisible reports whether the spinner is currently showing.
func (s *SpinnerModel) IsVisible() bool { return s.visible }

// SetTokenCount updates the displayed token count.
func (s *SpinnerModel) SetTokenCount(n int) { s.tokenCount = n }

// SetLabel updates the spinner label text.
func (s *SpinnerModel) SetLabel(label string) { s.label = label }

// Init returns the initial tick command.
func (s *SpinnerModel) Init() tea.Cmd {
	if !s.visible {
		return nil
	}
	return s.tickCmd()
}

// Update processes tick messages and advances the animation frame.
func (s *SpinnerModel) Update(msg tea.Msg) (*SpinnerModel, tea.Cmd) {
	if !s.visible {
		return s, nil
	}
	if _, ok := msg.(spinnerTickMsg); ok {
		s.currentMs = time.Since(s.startTime).Milliseconds()
		s.frameIdx++
		return s, s.tickCmd()
	}
	return s, nil
}

// View renders the full spinner row:
//
//	● Thinking…                          42 tokens · 3s
func (s *SpinnerModel) View() string {
	if !s.visible {
		return ""
	}

	var sb strings.Builder

	// 1. Spinner glyph
	sb.WriteString(s.renderGlyph())
	sb.WriteString(" ")

	// 2. Label with shimmer effect
	sb.WriteString(s.renderLabel())

	// 3. Right-side status (tokens + elapsed)
	status := s.renderStatus()
	if status != "" {
		sb.WriteString("  ")
		sb.WriteString(status)
	}

	return sb.String()
}

// ── Internal rendering ──────────────────────────────────────────────────────

func (s *SpinnerModel) tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

// stalledIntensity returns 0.0-1.0 representing how "stalled" the spinner is.
// 0 = not stalled, 1 = fully stalled (red).
func (s *SpinnerModel) stalledIntensity() float64 {
	if s.currentMs <= stalledThresholdMs {
		return 0
	}
	t := float64(s.currentMs-stalledThresholdMs) / float64(stalledDurationMs)
	if t > 1 {
		t = 1
	}
	return t
}

// renderGlyph renders the animated spinner character with color interpolation.
func (s *SpinnerModel) renderGlyph() string {
	if s.reducedMotion {
		style := lipgloss.NewStyle().Foreground(color.Resolve(s.theme.Claude))
		return style.Render("●")
	}

	char := s.frames[s.frameIdx%len(s.frames)]
	intensity := s.stalledIntensity()

	if intensity > 0 {
		// Interpolate from Claude color to Error red
		baseRGB, baseOk := color.ParseRGB(s.theme.Claude)
		errRGB, errOk := color.ParseRGB(s.theme.Error)
		if baseOk && errOk {
			blended := color.Interpolate(baseRGB, errRGB, intensity)
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(blended.ToHex()))
			return style.Render(char)
		}
	}

	style := lipgloss.NewStyle().Foreground(color.Resolve(s.theme.Claude))
	return style.Render(char)
}

// renderLabel renders the label text with shimmer effect.
func (s *SpinnerModel) renderLabel() string {
	if s.reducedMotion || s.label == "" {
		dimStyle := lipgloss.NewStyle().Foreground(color.Resolve(s.theme.Inactive))
		return dimStyle.Render(s.label)
	}

	return ShimmerText(s.label, s.theme.Claude, s.theme.ClaudeShimmer, s.currentMs)
}

// renderStatus renders the right-side token count and elapsed time.
func (s *SpinnerModel) renderStatus() string {
	dimStyle := lipgloss.NewStyle().Foreground(color.Resolve(s.theme.Inactive))

	var parts []string

	if s.tokenCount > 0 {
		parts = append(parts, fmt.Sprintf("%d tokens", s.tokenCount))
	}

	// Only show elapsed time after 500ms
	if s.currentMs > 500 {
		elapsed := time.Duration(s.currentMs) * time.Millisecond
		parts = append(parts, formatElapsed(elapsed))
	}

	if len(parts) == 0 {
		return ""
	}

	return dimStyle.Render(strings.Join(parts, " · "))
}

// formatElapsed formats a duration for display (e.g. "3s", "1m 5s").
func formatElapsed(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) - m*60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}
