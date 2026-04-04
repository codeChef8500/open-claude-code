package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wall-ai/agent-engine/internal/tui/spinnerv2"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
)

// SpinnerMode re-exports spinnerv2.SpinnerMode for use in the tui package.
type SpinnerMode = spinnerv2.SpinnerMode

const (
	SpinnerModeRequesting = spinnerv2.ModeRequesting
	SpinnerModeThinking   = spinnerv2.ModeThinking
	SpinnerModeToolUse    = spinnerv2.ModeToolUse
)

// EffortLevel re-exports spinnerv2.EffortLevel for use in the tui package.
type EffortLevel = spinnerv2.EffortLevel

const (
	EffortNone   = spinnerv2.EffortNone
	EffortLow    = spinnerv2.EffortLow
	EffortMedium = spinnerv2.EffortMedium
	EffortHigh   = spinnerv2.EffortHigh
	EffortMax    = spinnerv2.EffortMax
)

// SpinnerModel wraps spinnerv2.SpinnerModel, providing the same value-type
// interface expected by App. Under the hood it uses the custom frame animation,
// shimmer color effects, stalled detection, and random verbs from spinnerv2.
type SpinnerModel struct {
	inner *spinnerv2.SpinnerModel
}

// NewSpinner creates a SpinnerModel backed by spinnerv2.
func NewSpinner(theme Theme) SpinnerModel {
	// Build a themes.Theme with just the colors the spinner needs.
	t := themes.Theme{
		Claude:        "#d77757",
		ClaudeShimmer: "#e8a98a",
		Error:         "#ff6b80",
		Inactive:      "#666666",
	}
	return SpinnerModel{inner: spinnerv2.New(t)}
}

// NewSpinnerWithTheme creates a SpinnerModel using a full themes.Theme.
func NewSpinnerWithTheme(t themes.Theme) SpinnerModel {
	return SpinnerModel{inner: spinnerv2.New(t)}
}

// Show makes the spinner visible with the given label.
func (s *SpinnerModel) Show(label string) {
	s.inner.Show(label)
}

// ShowWithMode makes the spinner visible with a label and specific mode.
func (s *SpinnerModel) ShowWithMode(label string, mode SpinnerMode) {
	s.inner.ShowWithMode(label, mode)
}

// ShowRandom makes the spinner visible with a random verb.
func (s *SpinnerModel) ShowRandom() {
	s.inner.ShowRandom()
}

// Hide stops displaying the spinner.
func (s *SpinnerModel) Hide() {
	s.inner.Hide()
}

// IsVisible reports whether the spinner is currently shown.
func (s SpinnerModel) IsVisible() bool { return s.inner.IsVisible() }

// SetTokenCount updates the displayed token count.
func (s *SpinnerModel) SetTokenCount(n int) { s.inner.SetTokenCount(n) }

// SetLabel updates the spinner label text.
func (s *SpinnerModel) SetLabel(label string) { s.inner.SetLabel(label) }

// SetMode changes the spinner's visual mode without resetting animation.
func (s *SpinnerModel) SetMode(mode SpinnerMode) { s.inner.SetMode(mode) }

// Mode returns the current spinner mode.
func (s SpinnerModel) Mode() SpinnerMode { return s.inner.Mode() }

// Elapsed returns the duration since the spinner was shown.
func (s SpinnerModel) Elapsed() time.Duration { return s.inner.Elapsed() }

// SetEffort updates the effort level indicator.
func (s *SpinnerModel) SetEffort(e EffortLevel) { s.inner.SetEffort(e) }

// Effort returns the current effort level.
func (s SpinnerModel) Effort() EffortLevel { return s.inner.Effort() }

// Init returns the spinner tick command.
func (s SpinnerModel) Init() tea.Cmd {
	return s.inner.Init()
}

// Update forwards tick messages to the underlying spinner.
func (s SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	_, cmd := s.inner.Update(msg)
	return s, cmd
}

// View renders the spinner + label if visible, otherwise returns "".
func (s SpinnerModel) View() string {
	return s.inner.View()
}
