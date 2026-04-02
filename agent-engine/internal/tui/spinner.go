package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// SpinnerModel wraps bubbles/spinner with a label and theme-aware styling.
type SpinnerModel struct {
	inner   spinner.Model
	label   string
	visible bool
	theme   Theme
}

// NewSpinner creates a spinner using the Dots style.
func NewSpinner(theme Theme) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.Spinner
	return SpinnerModel{inner: s, theme: theme}
}

// Show makes the spinner visible with the given label.
func (s *SpinnerModel) Show(label string) {
	s.label = label
	s.visible = true
}

// Hide stops displaying the spinner.
func (s *SpinnerModel) Hide() {
	s.visible = false
	s.label = ""
}

// IsVisible reports whether the spinner is currently shown.
func (s SpinnerModel) IsVisible() bool { return s.visible }

// Init returns the spinner tick command.
func (s SpinnerModel) Init() tea.Cmd {
	return s.inner.Tick
}

// Update forwards tick messages to the underlying spinner.
func (s SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	var cmd tea.Cmd
	s.inner, cmd = s.inner.Update(msg)
	return s, cmd
}

// View renders the spinner + label if visible, otherwise returns "".
func (s SpinnerModel) View() string {
	if !s.visible {
		return ""
	}
	return s.inner.View() + " " + s.theme.Dimmed.Render(s.label)
}
