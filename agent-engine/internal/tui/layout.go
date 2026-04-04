package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Region identifies a named area in the three-region layout.
type Region int

const (
	RegionHeader Region = iota // StatusLine (1-2 rows)
	RegionBody                 // Message list (scrollable)
	RegionFooter               // Input + footer bar
)

// Layout manages the three-region terminal layout:
//
//	┌── header (statusline) ──┐
//	│ body (messages + spin)  │
//	├── input ────────────────┤
//	│ footer (status bar)     │
//	└─────────────────────────┘
type Layout struct {
	width  int
	height int

	headerHeight int // reserved for status line
	footerHeight int // reserved for input + footer bar
	inputHeight  int // multi-line input area

	// Computed body height
	bodyHeight int
}

const defaultInputH = 5 // 1 top border + 3 textarea lines + 1 padding

// NewLayout creates a layout with default region sizes.
func NewLayout(width, height int) Layout {
	l := Layout{
		headerHeight: 1,
		footerHeight: 1,
		inputHeight:  defaultInputH,
	}
	l.Resize(width, height)
	return l
}

// defaultInputHeight returns the baseline input region height (without popup).
func (l *Layout) defaultInputHeight() int { return defaultInputH }

// Resize recalculates all region dimensions.
func (l *Layout) Resize(width, height int) {
	l.width = width
	l.height = height

	// body = total - header - input - footer - gap
	body := height - l.headerHeight - l.inputHeight - l.footerHeight - 1
	if body < 3 {
		body = 3
	}
	l.bodyHeight = body
}

// Width returns the terminal width.
func (l *Layout) Width() int { return l.width }

// Height returns the terminal height.
func (l *Layout) Height() int { return l.height }

// BodyWidth returns the usable width inside the body region.
func (l *Layout) BodyWidth() int { return l.width }

// BodyHeight returns the computed body region height.
func (l *Layout) BodyHeight() int { return l.bodyHeight }

// InputHeight returns the input area height.
func (l *Layout) InputHeight() int { return l.inputHeight }

// SetInputHeight changes the input region height (e.g. for multi-line expand).
func (l *Layout) SetInputHeight(h int) {
	if h < 1 {
		h = 1
	}
	if h > l.height/2 {
		h = l.height / 2
	}
	l.inputHeight = h
	l.Resize(l.width, l.height)
}

// Compose joins header, body, input, and footer into a single full-screen view.
// No explicit separator — the input's top-only round border provides visual separation
// (matching claude-code-main's layout).
func (l *Layout) Compose(header, body, input, footer string) string {
	// Pad/truncate each region to its allocated height
	headerView := padToHeight(header, l.headerHeight, l.width)
	bodyView := padToHeight(body, l.bodyHeight, l.width)
	inputView := padToHeight(input, l.inputHeight, l.width)
	footerView := padToHeight(footer, l.footerHeight, l.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		headerView,
		bodyView,
		inputView,
		footerView,
	)
}

// padToHeight ensures a string block is exactly `h` lines tall and `w` wide.
func padToHeight(content string, h, w int) string {
	lines := strings.Split(content, "\n")

	// Truncate if too many lines
	if len(lines) > h {
		lines = lines[len(lines)-h:]
	}

	// Pad with empty lines if too few
	for len(lines) < h {
		lines = append(lines, "")
	}

	// Ensure each line is padded to width
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < w {
			lines[i] = line + strings.Repeat(" ", w-lineWidth)
		}
	}

	return strings.Join(lines, "\n")
}
