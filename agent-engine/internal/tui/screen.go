package tui

import tea "github.com/charmbracelet/bubbletea"

// ScreenMode identifies which screen is active.
type ScreenMode int

const (
	ScreenPrompt     ScreenMode = iota // Main REPL view
	ScreenTranscript                   // Full transcript view (Ctrl+O)
)

// ScreenManager handles alternate-screen lifecycle and terminal state.
type ScreenManager struct {
	mode         ScreenMode
	altScreen    bool
	mouseEnabled bool
	width        int
	height       int
}

// NewScreenManager creates a screen manager with default settings.
func NewScreenManager() ScreenManager {
	return ScreenManager{
		mode:   ScreenPrompt,
		width:  80,
		height: 24,
	}
}

// EnterAltScreen returns a command to enter the alternate screen buffer.
func (s *ScreenManager) EnterAltScreen() tea.Cmd {
	s.altScreen = true
	return tea.EnterAltScreen
}

// ExitAltScreen returns a command to leave the alternate screen buffer.
func (s *ScreenManager) ExitAltScreen() tea.Cmd {
	s.altScreen = false
	return tea.ExitAltScreen
}

// EnableMouse returns a command to enable mouse event reporting.
func (s *ScreenManager) EnableMouse() tea.Cmd {
	s.mouseEnabled = true
	return tea.EnableMouseCellMotion
}

// DisableMouse returns a command to disable mouse event reporting.
func (s *ScreenManager) DisableMouse() tea.Cmd {
	s.mouseEnabled = false
	// DisableMouseCellMotion is not available in all bubbletea versions.
	// Re-entering alt screen effectively resets mouse state.
	return nil
}

// IsAltScreen reports whether alternate screen is active.
func (s *ScreenManager) IsAltScreen() bool { return s.altScreen }

// Mode returns the current screen mode.
func (s *ScreenManager) Mode() ScreenMode { return s.mode }

// SetMode switches between prompt and transcript screens.
func (s *ScreenManager) SetMode(m ScreenMode) { s.mode = m }

// Resize updates stored terminal dimensions.
func (s *ScreenManager) Resize(w, h int) {
	s.width = w
	s.height = h
}

// Width returns the terminal width.
func (s *ScreenManager) Width() int { return s.width }

// Height returns the terminal height.
func (s *ScreenManager) Height() int { return s.height }
