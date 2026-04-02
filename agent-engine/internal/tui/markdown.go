package tui

import (
	"sync"

	"github.com/charmbracelet/glamour"
)

// MarkdownRenderer wraps glamour for thread-safe markdown→ANSI rendering.
// A single renderer instance is reused across messages to avoid repeated
// style parsing.
type MarkdownRenderer struct {
	mu       sync.Mutex
	renderer *glamour.TermRenderer
	width    int
	dark     bool
}

// NewMarkdownRenderer creates a renderer for the given terminal width.
// dark controls whether the dark or light style is used.
func NewMarkdownRenderer(width int, dark bool) (*MarkdownRenderer, error) {
	style := glamour.WithAutoStyle()
	if dark {
		style = glamour.WithStylePath("dark")
	} else {
		style = glamour.WithStylePath("light")
	}
	r, err := glamour.NewTermRenderer(
		style,
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		return nil, err
	}
	return &MarkdownRenderer{renderer: r, width: width, dark: dark}, nil
}

// Render converts markdown text to ANSI-escaped terminal output.
// Falls back to the raw input on renderer error.
func (m *MarkdownRenderer) Render(md string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out, err := m.renderer.Render(md)
	if err != nil {
		return md
	}
	return out
}

// Resize recreates the renderer at the new width.
func (m *MarkdownRenderer) Resize(width int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if width == m.width {
		return nil
	}
	style := glamour.WithStylePath("dark")
	if !m.dark {
		style = glamour.WithStylePath("light")
	}
	r, err := glamour.NewTermRenderer(
		style,
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		return err
	}
	m.renderer = r
	m.width = width
	return nil
}
