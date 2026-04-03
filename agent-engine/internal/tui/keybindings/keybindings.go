package keybindings

import "github.com/charmbracelet/bubbles/key"

// Mode represents the keybinding mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeVimNormal
	ModeVimInsert
)

// ExtendedKeyMap extends the base keymap with additional bindings
// for vim mode, search, and advanced navigation.
type ExtendedKeyMap struct {
	// Basic input
	Send       key.Binding
	Newline    key.Binding
	Quit       key.Binding
	ForceQuit  key.Binding

	// Scrolling
	ScrollUp   key.Binding
	ScrollDown key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	GotoTop    key.Binding
	GotoBottom key.Binding

	// Commands
	Compact    key.Binding
	Clear      key.Binding
	ToggleHelp key.Binding
	Search     key.Binding
	Transcript key.Binding

	// Permission dialog
	YesConfirm    key.Binding
	NoConfirm     key.Binding
	AlwaysAllow   key.Binding
	AlwaysDeny    key.Binding

	// Completion
	CompletionNext key.Binding
	CompletionPrev key.Binding
	CompletionAccept key.Binding
	CompletionDismiss key.Binding

	// Vim mode
	VimEscape    key.Binding
	VimInsert    key.Binding
	VimAppend    key.Binding
	VimDown      key.Binding
	VimUp        key.Binding
	VimLeft      key.Binding
	VimRight     key.Binding
	VimWordFwd   key.Binding
	VimWordBack  key.Binding
	VimLineStart key.Binding
	VimLineEnd   key.Binding
	VimDelete    key.Binding
	VimDeleteLine key.Binding
	VimPaste     key.Binding
	VimUndo      key.Binding

	// History
	HistoryPrev key.Binding
	HistoryNext key.Binding

	// Abort
	AbortQuery key.Binding
}

// DefaultExtendedKeyMap returns the full keybinding set.
func DefaultExtendedKeyMap() ExtendedKeyMap {
	return ExtendedKeyMap{
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
		Newline: key.NewBinding(
			key.WithKeys("alt+enter", "shift+enter"),
			key.WithHelp("alt+enter", "newline"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+\\"),
			key.WithHelp("ctrl+\\", "force quit"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("ctrl+up"),
			key.WithHelp("ctrl+↑", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("ctrl+down"),
			key.WithHelp("ctrl+↓", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		GotoTop: key.NewBinding(
			key.WithKeys("ctrl+home"),
			key.WithHelp("ctrl+home", "top"),
		),
		GotoBottom: key.NewBinding(
			key.WithKeys("ctrl+end"),
			key.WithHelp("ctrl+end", "bottom"),
		),
		Compact: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("ctrl+k", "compact"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "clear"),
		),
		ToggleHelp: key.NewBinding(
			key.WithKeys("ctrl+?", "f1"),
			key.WithHelp("f1", "help"),
		),
		Search: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("ctrl+f", "search"),
		),
		Transcript: key.NewBinding(
			key.WithKeys("ctrl+o"),
			key.WithHelp("ctrl+o", "transcript"),
		),
		YesConfirm: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y", "allow"),
		),
		NoConfirm: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n", "deny"),
		),
		AlwaysAllow: key.NewBinding(
			key.WithKeys("a", "A"),
			key.WithHelp("a", "always allow"),
		),
		AlwaysDeny: key.NewBinding(
			key.WithKeys("d", "D"),
			key.WithHelp("d", "always deny"),
		),
		CompletionNext: key.NewBinding(
			key.WithKeys("tab", "down"),
			key.WithHelp("tab", "next suggestion"),
		),
		CompletionPrev: key.NewBinding(
			key.WithKeys("shift+tab", "up"),
			key.WithHelp("shift+tab", "prev suggestion"),
		),
		CompletionAccept: key.NewBinding(
			key.WithKeys("enter", "tab"),
			key.WithHelp("enter", "accept"),
		),
		CompletionDismiss: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "dismiss"),
		),
		VimEscape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "normal mode"),
		),
		VimInsert: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "insert mode"),
		),
		VimAppend: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "append"),
		),
		VimDown: key.NewBinding(
			key.WithKeys("j"),
			key.WithHelp("j", "down"),
		),
		VimUp: key.NewBinding(
			key.WithKeys("k"),
			key.WithHelp("k", "up"),
		),
		VimLeft: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "left"),
		),
		VimRight: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "right"),
		),
		VimWordFwd: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "word forward"),
		),
		VimWordBack: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "word back"),
		),
		VimLineStart: key.NewBinding(
			key.WithKeys("0", "^"),
			key.WithHelp("0", "line start"),
		),
		VimLineEnd: key.NewBinding(
			key.WithKeys("$"),
			key.WithHelp("$", "line end"),
		),
		VimDelete: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "delete char"),
		),
		VimDeleteLine: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("dd", "delete line"),
		),
		VimPaste: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "paste"),
		),
		VimUndo: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "undo"),
		),
		HistoryPrev: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "previous input"),
		),
		HistoryNext: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "next input"),
		),
		AbortQuery: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "abort query"),
		),
	}
}

// ShortHelp returns the compact help bindings.
func (k ExtendedKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Send, k.Newline, k.ScrollUp, k.ScrollDown, k.Quit}
}

// FullHelp returns all bindings grouped for the help view.
func (k ExtendedKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Send, k.Newline, k.Quit, k.ForceQuit},
		{k.ScrollUp, k.ScrollDown, k.PageUp, k.PageDown, k.GotoTop, k.GotoBottom},
		{k.Compact, k.Clear, k.Search, k.Transcript, k.ToggleHelp},
		{k.AbortQuery, k.HistoryPrev, k.HistoryNext},
	}
}

// VimHelp returns vim-specific bindings for the help view.
func (k ExtendedKeyMap) VimHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.VimEscape, k.VimInsert, k.VimAppend},
		{k.VimUp, k.VimDown, k.VimLeft, k.VimRight},
		{k.VimWordFwd, k.VimWordBack, k.VimLineStart, k.VimLineEnd},
		{k.VimDelete, k.VimDeleteLine, k.VimPaste, k.VimUndo},
	}
}

// RenderHelp renders a formatted help view.
func RenderHelp(km ExtendedKeyMap, vimMode bool, width int) string {
	groups := km.FullHelp()
	if vimMode {
		groups = append(groups, km.VimHelp()...)
	}

	var lines []string
	for _, group := range groups {
		var parts []string
		for _, b := range group {
			h := b.Help()
			parts = append(parts, h.Key+": "+h.Desc)
		}
		if len(parts) > 0 {
			lines = append(lines, "  "+joinMax(parts, "  ", width-4))
		}
	}

	return "Keybindings:\n" + joinLines(lines)
}

func joinMax(parts []string, sep string, maxWidth int) string {
	var result string
	for i, p := range parts {
		if i > 0 {
			candidate := result + sep + p
			if len(candidate) > maxWidth {
				result += "\n  " + p
				continue
			}
			result = candidate
		} else {
			result = p
		}
	}
	return result
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
