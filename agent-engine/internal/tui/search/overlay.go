package search

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Overlay is a search bar overlay for finding text in the message history.
type Overlay struct {
	visible    bool
	query      string
	results    []Hit
	current    int
	totalHits  int
	width      int

	// Styles
	barStyle     lipgloss.Style
	inputStyle   lipgloss.Style
	matchStyle   lipgloss.Style
	noMatchStyle lipgloss.Style
	dimStyle     lipgloss.Style
}

// Hit represents a search match.
type Hit struct {
	MessageIdx int
	Offset     int
	Line       int
	Context    string // surrounding text
}

// SearchFn is called when the search query changes.
type SearchFn func(query string) []Hit

// NewOverlay creates a search overlay.
func NewOverlay(width int) *Overlay {
	return &Overlay{
		width: width,
		barStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("250")).
			Padding(0, 1),
		inputStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true),
		matchStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")),
		noMatchStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")),
		dimStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
	}
}

// Show activates the search overlay.
func (o *Overlay) Show() {
	o.visible = true
	o.query = ""
	o.results = nil
	o.current = 0
	o.totalHits = 0
}

// Hide deactivates the search overlay.
func (o *Overlay) Hide() {
	o.visible = false
	o.query = ""
	o.results = nil
}

// IsVisible returns whether the overlay is active.
func (o *Overlay) IsVisible() bool { return o.visible }

// Query returns the current search query.
func (o *Overlay) Query() string { return o.query }

// CurrentHit returns the currently selected hit, or nil.
func (o *Overlay) CurrentHit() *Hit {
	if len(o.results) == 0 {
		return nil
	}
	return &o.results[o.current]
}

// SetWidth updates the overlay width.
func (o *Overlay) SetWidth(w int) { o.width = w }

// Update handles key events for the search overlay.
// Returns updated overlay and whether the key was consumed.
func (o *Overlay) Update(msg tea.KeyMsg, searchFn SearchFn) bool {
	if !o.visible {
		return false
	}

	switch msg.Type {
	case tea.KeyEscape:
		o.Hide()
		return true

	case tea.KeyEnter:
		// Confirm and close, keeping current position
		o.visible = false
		return true

	case tea.KeyBackspace:
		if len(o.query) > 0 {
			o.query = o.query[:len(o.query)-1]
			o.doSearch(searchFn)
		}
		return true

	case tea.KeyUp, tea.KeyCtrlP:
		o.prevResult()
		return true

	case tea.KeyDown, tea.KeyCtrlN:
		o.nextResult()
		return true

	case tea.KeyRunes:
		o.query += msg.String()
		o.doSearch(searchFn)
		return true
	}

	return false
}

func (o *Overlay) doSearch(searchFn SearchFn) {
	if searchFn == nil || o.query == "" {
		o.results = nil
		o.totalHits = 0
		o.current = 0
		return
	}
	o.results = searchFn(o.query)
	o.totalHits = len(o.results)
	o.current = 0
}

func (o *Overlay) nextResult() {
	if len(o.results) == 0 {
		return
	}
	o.current = (o.current + 1) % len(o.results)
}

func (o *Overlay) prevResult() {
	if len(o.results) == 0 {
		return
	}
	o.current--
	if o.current < 0 {
		o.current = len(o.results) - 1
	}
}

// View renders the search overlay bar.
func (o *Overlay) View() string {
	if !o.visible {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(o.inputStyle.Render("/ "))
	sb.WriteString(o.inputStyle.Render(o.query))
	sb.WriteString(o.dimStyle.Render("█")) // cursor

	if o.query != "" {
		sb.WriteString("  ")
		if o.totalHits > 0 {
			sb.WriteString(o.matchStyle.Render(
				strings.Repeat(" ", 1) + // padding
					string(rune('0'+o.current+1)) + "/" +
					string(rune('0'+o.totalHits))))
		} else {
			sb.WriteString(o.noMatchStyle.Render("no matches"))
		}
	}

	return o.barStyle.Width(o.width).Render(sb.String())
}
