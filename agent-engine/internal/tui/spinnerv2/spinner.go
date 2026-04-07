package spinnerv2

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// SpinnerMode distinguishes visual states matching claude-code-main's
// SpinnerAnimationRow modes.
type SpinnerMode int

const (
	// ModeRequesting — fast shimmer, Claude color. Used when waiting for API response.
	ModeRequesting SpinnerMode = iota
	// ModeThinking — grey/inactive shimmer. Used during extended thinking.
	ModeThinking
	// ModeToolUse — pulsing glyph. Used while a tool is executing.
	ModeToolUse
)

// tokenShowThresholdMs is the delay before token count is displayed (30s).
const tokenShowThresholdMs = 30000

// spinnerTickMsg is the internal tick message for animation frames.
type spinnerTickMsg time.Time

// SpinnerModel is a custom spinner matching claude-code-main's animation:
//   - Frame sequence: ·✢✳✶✻✽ (forward + reverse oscillation)
//   - 120ms frame interval
//   - Stalled detection: after 5s, color transitions from Claude→Error (red)
//   - Shimmer text effect on the label
//   - Token counter and elapsed time display
//   - Three visual modes: requesting, thinking, tool-use
//
// EffortLevel represents the API effort/budget level.
type EffortLevel int

const (
	EffortNone   EffortLevel = iota // no indicator
	EffortLow                       // ○
	EffortMedium                    // ◐
	EffortHigh                      // ●
	EffortMax                       // ◉
)

// EffortGlyph returns the effort indicator character.
func (e EffortLevel) Glyph() string {
	switch e {
	case EffortLow:
		return "○"
	case EffortMedium:
		return "◐"
	case EffortHigh:
		return "●"
	case EffortMax:
		return "◉"
	default:
		return ""
	}
}

// minThinkingDisplayMs is the minimum time to show "thinking" before switching.
const minThinkingDisplayMs = 2000

type SpinnerModel struct {
	frames  []string
	visible bool
	label   string
	mode    SpinnerMode
	effort  EffortLevel
	theme   themes.Theme

	startTime  time.Time
	currentMs  int64 // elapsed ms since animation start
	frameIdx   int
	tokenCount int

	// Smooth token animation (matching claude-code-main's SpinnerAnimationRow)
	displayedTokens int

	// activeFormOverride, when set, replaces the spinner label with the
	// current todo's activeForm text (e.g. "Running tests…").
	activeFormOverride string

	// thinkingStartMs records when ModeThinking was entered, used for
	// minimum 2s display time.
	thinkingStartMs int64

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

// Show makes the spinner visible with a new label and mode.
func (s *SpinnerModel) Show(label string) {
	s.visible = true
	s.label = label
	s.mode = ModeRequesting
	s.startTime = time.Now()
	s.currentMs = 0
	s.frameIdx = 0
	s.tokenCount = 0
	s.displayedTokens = 0
}

// ShowWithMode shows the spinner with a specific visual mode.
func (s *SpinnerModel) ShowWithMode(label string, mode SpinnerMode) {
	s.Show(label)
	s.mode = mode
}

// SetMode changes the spinner's visual mode without resetting animation.
// ModeThinking tracks its start time for minimum display duration.
func (s *SpinnerModel) SetMode(mode SpinnerMode) {
	if mode == ModeThinking && s.mode != ModeThinking {
		s.thinkingStartMs = s.currentMs
	}
	s.mode = mode
}

// SetActiveForm sets (or clears) the todo activeForm override.
// When non-empty, the spinner label shows this text + "…" suffix.
func (s *SpinnerModel) SetActiveForm(form string) {
	s.activeFormOverride = form
}

// ActiveForm returns the current activeForm override.
func (s *SpinnerModel) ActiveForm() string {
	return s.activeFormOverride
}

// Mode returns the current spinner mode.
func (s *SpinnerModel) Mode() SpinnerMode {
	return s.mode
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
	s.displayedTokens = 0
}

// IsVisible reports whether the spinner is currently showing.
func (s *SpinnerModel) IsVisible() bool { return s.visible }

// SetTokenCount updates the displayed token count.
func (s *SpinnerModel) SetTokenCount(n int) { s.tokenCount = n }

// SetLabel updates the spinner label text.
// Note: if activeFormOverride is set, it takes precedence in View().
func (s *SpinnerModel) SetLabel(label string) { s.label = label }

// SetEffort updates the effort level indicator.
func (s *SpinnerModel) SetEffort(e EffortLevel) { s.effort = e }

// Effort returns the current effort level.
func (s *SpinnerModel) Effort() EffortLevel { return s.effort }

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
		// Smooth token count animation
		s.advanceTokenAnimation()
		return s, s.tickCmd()
	}
	return s, nil
}

// Elapsed returns the duration since the spinner was shown.
func (s *SpinnerModel) Elapsed() time.Duration {
	if !s.visible {
		return 0
	}
	return time.Since(s.startTime)
}

// advanceTokenAnimation smoothly increments displayed tokens toward target,
// matching claude-code-main's SpinnerAnimationRow logic.
func (s *SpinnerModel) advanceTokenAnimation() {
	if s.displayedTokens >= s.tokenCount {
		s.displayedTokens = s.tokenCount
		return
	}
	gap := s.tokenCount - s.displayedTokens
	var inc int
	switch {
	case gap < 70:
		inc = 3
	case gap < 300:
		inc = 10
	default:
		inc = 30
	}
	s.displayedTokens += inc
	if s.displayedTokens > s.tokenCount {
		s.displayedTokens = s.tokenCount
	}
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

	// 2. Label with shimmer effect (activeForm override takes precedence)
	label := s.effectiveLabel()
	sb.WriteString(s.renderLabelText(label))

	// 2b. Effort level indicator
	if g := s.effort.Glyph(); g != "" {
		sb.WriteString(" ")
		dimStyle := lipgloss.NewStyle().Foreground(color.Resolve(s.theme.Inactive))
		sb.WriteString(dimStyle.Render(g))
	}

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
// In ModeToolUse, the glyph pulses via a sine-based opacity.
func (s *SpinnerModel) renderGlyph() string {
	if s.reducedMotion {
		style := lipgloss.NewStyle().Foreground(color.Resolve(s.theme.Claude))
		return style.Render("●")
	}

	char := s.frames[s.frameIdx%len(s.frames)]

	// ModeToolUse: pulse the glyph via sine-based color fade
	if s.mode == ModeToolUse {
		baseRGB, baseOk := color.ParseRGB(s.theme.Claude)
		dimRGB, dimOk := color.ParseRGB(s.theme.Inactive)
		if baseOk && dimOk {
			// Smooth sine pulse: map sin [-1,1] to [0, 0.5]
			phase := float64(s.currentMs) / 500.0
			t := (math.Sin(phase) + 1.0) / 4.0 // range [0, 0.5]
			blended := color.Interpolate(baseRGB, dimRGB, t)
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(blended.ToHex()))
			return style.Render(char)
		}
	}

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

// effectiveLabel returns the label to display, preferring activeFormOverride.
func (s *SpinnerModel) effectiveLabel() string {
	if s.activeFormOverride != "" {
		af := s.activeFormOverride
		if !strings.HasSuffix(af, "…") && !strings.HasSuffix(af, "...") {
			af += "…"
		}
		return af
	}
	return s.label
}

// renderLabelText renders the given label text with shimmer effect.
// ModeThinking uses Inactive/InactiveShimmer colors.
// ModeRequesting uses Claude/ClaudeShimmer colors.
func (s *SpinnerModel) renderLabelText(label string) string {
	if s.reducedMotion || label == "" {
		dimStyle := lipgloss.NewStyle().Foreground(color.Resolve(s.theme.Inactive))
		return dimStyle.Render(label)
	}

	switch s.mode {
	case ModeThinking:
		// Show thinking duration after minimum display time.
		if s.currentMs-s.thinkingStartMs >= minThinkingDisplayMs {
			thinkSec := (s.currentMs - s.thinkingStartMs) / 1000
			label = fmt.Sprintf("thinking… (%ds)", thinkSec)
		}
		return ShimmerText(label, s.theme.Inactive, s.theme.InactiveShimmer, s.currentMs)
	default:
		return ShimmerText(label, s.theme.Claude, s.theme.ClaudeShimmer, s.currentMs)
	}
}

// renderStatus renders the right-side token count and elapsed time.
// Token count is only shown after tokenShowThresholdMs (30s), matching claude-code.
func (s *SpinnerModel) renderStatus() string {
	dimStyle := lipgloss.NewStyle().Foreground(color.Resolve(s.theme.Inactive))

	var parts []string

	// Only show token count after 30s (matching claude-code threshold)
	if s.displayedTokens > 0 && s.currentMs > tokenShowThresholdMs {
		parts = append(parts, fmt.Sprintf("%d tokens", s.displayedTokens))
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
