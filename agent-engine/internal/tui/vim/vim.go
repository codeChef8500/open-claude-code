package vim

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Mode represents the current vim mode.
type Mode int

const (
	ModeInsert Mode = iota
	ModeNormal
	ModeVisual
	ModeCommand // : command line
)

// String returns the display name for the mode.
func (m Mode) String() string {
	switch m {
	case ModeInsert:
		return "INSERT"
	case ModeNormal:
		return "NORMAL"
	case ModeVisual:
		return "VISUAL"
	case ModeCommand:
		return "COMMAND"
	default:
		return "UNKNOWN"
	}
}

// VimState tracks the vim emulation state machine.
type VimState struct {
	Mode           Mode
	Enabled        bool
	PendingOp      string // partial operator (d, c, y, etc.)
	Register       string // yank register content
	Count          int    // numeric prefix
	LastSearch     string
	CommandBuffer  string // : command line buffer
}

// New creates a new vim state (starts in insert mode).
func New() *VimState {
	return &VimState{
		Mode:    ModeInsert,
		Enabled: false,
	}
}

// Toggle enables/disables vim mode. When disabled, all keys pass through.
func (v *VimState) Toggle() {
	v.Enabled = !v.Enabled
	if v.Enabled {
		v.Mode = ModeNormal
	} else {
		v.Mode = ModeInsert
	}
	v.PendingOp = ""
	v.Count = 0
}

// IsEnabled returns whether vim mode is active.
func (v *VimState) IsEnabled() bool {
	return v.Enabled
}

// Action represents a vim action to be applied to the input.
type Action struct {
	Type      ActionType
	Count     int
	Register  string
	Command   string // for :command
}

// ActionType classifies a vim action.
type ActionType int

const (
	ActionNone       ActionType = iota
	ActionPassthrough           // let the key pass to textarea
	ActionMoveLeft
	ActionMoveRight
	ActionMoveUp
	ActionMoveDown
	ActionMoveWordFwd
	ActionMoveWordBack
	ActionMoveLineStart
	ActionMoveLineEnd
	ActionMoveDocTop
	ActionMoveDocBottom
	ActionInsertMode
	ActionInsertLineStart
	ActionAppendMode
	ActionAppendLineEnd
	ActionNewLineBelow
	ActionNewLineAbove
	ActionDeleteChar
	ActionDeleteLine
	ActionDeleteToEnd
	ActionYankLine
	ActionPaste
	ActionPasteAbove
	ActionUndo
	ActionRedo
	ActionSearch
	ActionSearchNext
	ActionSearchPrev
	ActionEnterCommand
	ActionExecCommand
	ActionCancelCommand
	ActionVisualToggle
)

// HandleKey processes a key event in vim mode and returns the action.
// Returns ActionPassthrough if the key should be forwarded to textarea.
func (v *VimState) HandleKey(msg tea.KeyMsg) Action {
	if !v.Enabled {
		return Action{Type: ActionPassthrough}
	}

	switch v.Mode {
	case ModeNormal:
		return v.handleNormal(msg)
	case ModeInsert:
		return v.handleInsert(msg)
	case ModeVisual:
		return v.handleVisual(msg)
	case ModeCommand:
		return v.handleCommand(msg)
	default:
		return Action{Type: ActionPassthrough}
	}
}

func (v *VimState) handleNormal(msg tea.KeyMsg) Action {
	key := msg.String()

	// Numeric prefix
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' && v.PendingOp == "" {
		v.Count = v.Count*10 + int(key[0]-'0')
		return Action{Type: ActionNone}
	}
	if key == "0" && v.Count > 0 {
		v.Count = v.Count * 10
		return Action{Type: ActionNone}
	}

	count := v.Count
	if count == 0 {
		count = 1
	}
	v.Count = 0

	// Pending operator (d, c, y) + motion
	if v.PendingOp != "" {
		op := v.PendingOp
		v.PendingOp = ""
		switch op + key {
		case "dd":
			return Action{Type: ActionDeleteLine, Count: count}
		case "yy":
			return Action{Type: ActionYankLine, Count: count}
		default:
			return Action{Type: ActionNone}
		}
	}

	switch key {
	// Mode switches
	case "i":
		v.Mode = ModeInsert
		return Action{Type: ActionInsertMode}
	case "I":
		v.Mode = ModeInsert
		return Action{Type: ActionInsertLineStart}
	case "a":
		v.Mode = ModeInsert
		return Action{Type: ActionAppendMode}
	case "A":
		v.Mode = ModeInsert
		return Action{Type: ActionAppendLineEnd}
	case "o":
		v.Mode = ModeInsert
		return Action{Type: ActionNewLineBelow}
	case "O":
		v.Mode = ModeInsert
		return Action{Type: ActionNewLineAbove}
	case "v":
		v.Mode = ModeVisual
		return Action{Type: ActionVisualToggle}

	// Movement
	case "h", "left":
		return Action{Type: ActionMoveLeft, Count: count}
	case "l", "right":
		return Action{Type: ActionMoveRight, Count: count}
	case "j", "down":
		return Action{Type: ActionMoveDown, Count: count}
	case "k", "up":
		return Action{Type: ActionMoveUp, Count: count}
	case "w":
		return Action{Type: ActionMoveWordFwd, Count: count}
	case "b":
		return Action{Type: ActionMoveWordBack, Count: count}
	case "0", "^":
		return Action{Type: ActionMoveLineStart}
	case "$":
		return Action{Type: ActionMoveLineEnd}
	case "g":
		// gg = top
		v.PendingOp = "g"
		return Action{Type: ActionNone}
	case "G":
		return Action{Type: ActionMoveDocBottom}

	// Operators
	case "d":
		v.PendingOp = "d"
		return Action{Type: ActionNone}
	case "y":
		v.PendingOp = "y"
		return Action{Type: ActionNone}
	case "x":
		return Action{Type: ActionDeleteChar, Count: count}
	case "D":
		return Action{Type: ActionDeleteToEnd}
	case "p":
		return Action{Type: ActionPaste}
	case "P":
		return Action{Type: ActionPasteAbove}
	case "u":
		return Action{Type: ActionUndo}
	case "ctrl+r":
		return Action{Type: ActionRedo}

	// Search
	case "/":
		return Action{Type: ActionSearch}
	case "n":
		return Action{Type: ActionSearchNext}
	case "N":
		return Action{Type: ActionSearchPrev}

	// Command
	case ":":
		v.Mode = ModeCommand
		v.CommandBuffer = ""
		return Action{Type: ActionEnterCommand}

	default:
		return Action{Type: ActionNone}
	}
}

func (v *VimState) handleInsert(msg tea.KeyMsg) Action {
	switch msg.Type {
	case tea.KeyEscape:
		v.Mode = ModeNormal
		return Action{Type: ActionNone}
	default:
		return Action{Type: ActionPassthrough}
	}
}

func (v *VimState) handleVisual(msg tea.KeyMsg) Action {
	switch msg.String() {
	case "esc", "v":
		v.Mode = ModeNormal
		return Action{Type: ActionNone}
	case "y":
		v.Mode = ModeNormal
		return Action{Type: ActionYankLine}
	case "d":
		v.Mode = ModeNormal
		return Action{Type: ActionDeleteLine}
	default:
		// Movement in visual mode
		return v.handleNormal(msg)
	}
}

func (v *VimState) handleCommand(msg tea.KeyMsg) Action {
	switch msg.Type {
	case tea.KeyEscape:
		v.Mode = ModeNormal
		v.CommandBuffer = ""
		return Action{Type: ActionCancelCommand}
	case tea.KeyEnter:
		cmd := strings.TrimSpace(v.CommandBuffer)
		v.Mode = ModeNormal
		v.CommandBuffer = ""
		return Action{Type: ActionExecCommand, Command: cmd}
	case tea.KeyBackspace:
		if len(v.CommandBuffer) > 0 {
			v.CommandBuffer = v.CommandBuffer[:len(v.CommandBuffer)-1]
		}
		if len(v.CommandBuffer) == 0 {
			v.Mode = ModeNormal
			return Action{Type: ActionCancelCommand}
		}
		return Action{Type: ActionNone}
	default:
		if msg.Type == tea.KeyRunes {
			v.CommandBuffer += msg.String()
		}
		return Action{Type: ActionNone}
	}
}

// StatusText returns the current mode indicator for display.
func (v *VimState) StatusText() string {
	if !v.Enabled {
		return ""
	}
	text := "-- " + v.Mode.String() + " --"
	if v.PendingOp != "" {
		text += " " + v.PendingOp
	}
	if v.Count > 0 {
		text += " " + string(rune('0'+v.Count))
	}
	if v.Mode == ModeCommand {
		text = ":" + v.CommandBuffer
	}
	return text
}
