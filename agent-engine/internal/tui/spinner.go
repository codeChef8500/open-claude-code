package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wall-ai/agent-engine/internal/tui/spinnerv2"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
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
