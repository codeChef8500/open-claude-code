package events

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tui"
)

// Bridge converts engine stream events into Bubbletea messages and forwards
// them to the running program via program.Send.
//
// Usage:
//
//	b := events.NewBridge(program)
//	go b.DrainChannel(eventCh)
type Bridge struct {
	program *tea.Program
}

// NewBridge creates a Bridge that sends to the given Bubbletea program.
func NewBridge(p *tea.Program) *Bridge {
	return &Bridge{program: p}
}

// DrainChannel reads from an engine event channel and forwards each event
// as the appropriate Bubbletea message.  It blocks until the channel closes.
func (b *Bridge) DrainChannel(ch <-chan *engine.StreamEvent) {
	for ev := range ch {
		if ev == nil {
			continue
		}
		switch ev.Type {
		case engine.EventTextDelta:
			b.program.Send(tui.StreamTextMsg{Text: ev.Text})
		case engine.EventError:
			b.program.Send(tui.StreamErrorMsg{Err: fmt.Errorf("%s", ev.Error)})
		case engine.EventDone:
			b.program.Send(tui.StreamDoneMsg{})
		}
	}
	// Ensure we always signal done even if the channel closes without EventDone.
	b.program.Send(tui.StreamDoneMsg{})
}
