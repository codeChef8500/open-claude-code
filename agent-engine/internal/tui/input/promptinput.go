package input

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui"
	"github.com/wall-ai/agent-engine/internal/tui/color"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
)

// Mode identifies the current input mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeVim
	ModeMultiLine
	ModeSearch
)

// PromptInput is the enhanced input model for the TUI.
// It wraps bubbles/textarea with:
//   - Input history (up/down arrow)
//   - Slash command detection
//   - @-mention expansion
//   - Autocomplete popup
//   - Multi-line mode toggle (Alt+Enter)
//   - Vim mode toggle (/vim)
type PromptInput struct {
	textarea  textarea.Model
	history   *tui.InputHistory
	completer *tui.Completer
	compState tui.CompletionState

	mode      Mode
	vimMode   bool // persistent vim toggle
	width     int
	styles    themes.Styles
	themeData themes.Theme

	// SubmitFn is called when the user presses Enter.
	SubmitFn func(text string)
}

// NewPromptInput creates an enhanced input model.
func NewPromptInput(styles themes.Styles, themeData themes.Theme, completer *tui.Completer, width int) *PromptInput {
	ta := textarea.New()
	ta.Placeholder = "Reply to Claude\u2026"
	ta.Focus()
	ta.SetWidth(width)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0

	return &PromptInput{
		textarea:  ta,
		history:   tui.NewInputHistory(200),
		completer: completer,
		width:     width,
		styles:    styles,
		themeData: themeData,
	}
}

// Init returns the blink command for the cursor.
func (p *PromptInput) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles key events and returns commands.
func (p *PromptInput) Update(msg tea.Msg) (*PromptInput, tea.Cmd) {
	// If autocomplete is active, intercept Tab/Up/Down/Escape/Enter.
	if p.compState.Active {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.Type {
			case tea.KeyTab:
				if item := p.compState.SelectedItem(); item != nil {
					p.applyCompletion(item)
				}
				p.compState.Reset()
				return p, nil
			case tea.KeyUp:
				p.compState.SelectPrev()
				return p, nil
			case tea.KeyDown:
				p.compState.SelectNext()
				return p, nil
			case tea.KeyEscape:
				p.compState.Reset()
				return p, nil
			case tea.KeyEnter:
				if item := p.compState.SelectedItem(); item != nil {
					p.applyCompletion(item)
				}
				p.compState.Reset()
				return p, nil
			}
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		// Submit on Enter (not Alt+Enter)
		case msg.Type == tea.KeyEnter && !msg.Alt:
			text := strings.TrimSpace(p.textarea.Value())
			if text == "" {
				return p, nil
			}
			p.history.Add(text)
			p.textarea.Reset()
			p.compState.Reset()
			if p.SubmitFn != nil {
				p.SubmitFn(text)
			}
			return p, nil

		// History: Up arrow when on first line
		case msg.Type == tea.KeyUp:
			if p.textarea.Line() == 0 {
				val := p.textarea.Value()
				if prev, ok := p.history.Prev(val); ok {
					p.textarea.SetValue(prev)
					return p, nil
				}
			}

		// History: Down arrow when on last line
		case msg.Type == tea.KeyDown:
			if p.textarea.Line() >= p.textarea.LineCount()-1 {
				if next, ok := p.history.Next(); ok {
					p.textarea.SetValue(next)
					return p, nil
				}
			}

		// Tab: trigger completion
		case msg.Type == tea.KeyTab:
			p.triggerCompletion()
			return p, nil
		}
	}

	// Forward to textarea
	var cmd tea.Cmd
	p.textarea, cmd = p.textarea.Update(msg)

	// Auto-trigger completion on "/" at start
	val := p.textarea.Value()
	if strings.HasPrefix(val, "/") && len(val) >= 1 {
		p.triggerCompletion()
	} else if p.compState.Active && !strings.HasPrefix(val, "/") && !strings.Contains(val, "@") {
		p.compState.Reset()
	}

	return p, cmd
}

// View renders the input area including autocomplete popup.
// The input is wrapped in a top-only rounded border matching claude-code-main.
func (p *PromptInput) View() string {
	inputView := p.textarea.View()

	// Wrap in top-only round border (claude-code-main style)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color.Resolve(p.themeData.PromptBorder)).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		Width(p.width - 2)
	bordered := borderStyle.Render(inputView)

	if p.compState.Active && len(p.compState.Items) > 0 {
		popup := p.renderCompletionPopup()
		return lipgloss.JoinVertical(lipgloss.Left, popup, bordered)
	}

	return bordered
}

// Value returns the current input text.
func (p *PromptInput) Value() string {
	return p.textarea.Value()
}

// SetValue sets the input text.
func (p *PromptInput) SetValue(s string) {
	p.textarea.SetValue(s)
}

// Focus focuses the textarea.
func (p *PromptInput) Focus() tea.Cmd {
	return p.textarea.Focus()
}

// Blur unfocuses the textarea.
func (p *PromptInput) Blur() {
	p.textarea.Blur()
}

// Focused returns whether the textarea is focused.
func (p *PromptInput) Focused() bool {
	return p.textarea.Focused()
}

// SetWidth updates the input width.
func (p *PromptInput) SetWidth(w int) {
	p.width = w
	p.textarea.SetWidth(w)
}

// SetHeight updates the input height.
func (p *PromptInput) SetHeight(h int) {
	p.textarea.SetHeight(h)
}

// ToggleVimMode toggles vim-like keybindings.
func (p *PromptInput) ToggleVimMode() {
	p.vimMode = !p.vimMode
}

// IsVimMode returns whether vim mode is active.
func (p *PromptInput) IsVimMode() bool {
	return p.vimMode
}

// CursorLine returns the current cursor line.
func (p *PromptInput) CursorLine() int {
	return p.textarea.Line()
}

// ── Internal ─────────────────────────────────────────────────────────────────

func (p *PromptInput) triggerCompletion() {
	if p.completer == nil {
		return
	}
	val := p.textarea.Value()
	pos := len(val) // use end of input as cursor position
	items := p.completer.Complete(val, pos)
	if len(items) == 0 {
		p.compState.Reset()
		return
	}
	p.compState.Active = true
	p.compState.Items = items
	p.compState.Selected = 0
	p.compState.Prefix = val
}

func (p *PromptInput) applyCompletion(item *tui.CompletionItem) {
	// Replace the input with the completion value.
	if item.Kind == tui.CompletionCommand {
		p.textarea.SetValue(item.Value + " ")
	} else {
		// For @-mentions, replace from the @ character.
		val := p.textarea.Value()
		atIdx := strings.LastIndex(val, "@")
		if atIdx >= 0 {
			p.textarea.SetValue(val[:atIdx] + item.Value + " ")
		} else {
			p.textarea.SetValue(item.Value + " ")
		}
	}
}

func (p *PromptInput) renderCompletionPopup() string {
	maxShow := 8
	items := p.compState.Items
	if len(items) > maxShow {
		items = items[:maxShow]
	}

	var lines []string
	for i, item := range items {
		label := item.Label
		if item.Description != "" {
			label += "  " + p.styles.Dimmed.Render(item.Description)
		}
		if i == p.compState.Selected {
			label = p.styles.Highlight.Render("\u25b8 " + label)
		} else {
			label = "  " + label
		}
		lines = append(lines, label)
	}

	popup := strings.Join(lines, "\n")
	return p.styles.Border.Width(p.width - 4).Render(popup)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
