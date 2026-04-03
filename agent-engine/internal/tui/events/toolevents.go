package events

import (
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tui"
)

// ── Extended Bubbletea messages for tool use and status ──────────────────────

// CompactMsg signals a compaction event.
type CompactMsg struct {
	PreTokens  int
	PostTokens int
}

// PermissionRequestMsg asks the TUI to show a permission dialog.
type PermissionRequestMsg struct {
	ToolName    string
	Description string
	ResponseCh  chan<- bool
}

// ── Enhanced Bridge ─────────────────────────────────────────────────────────

// EnhancedBridge extends Bridge with tool use and usage event forwarding.
type EnhancedBridge struct {
	program *tea.Program
}

// NewEnhancedBridge creates an enhanced bridge.
func NewEnhancedBridge(p *tea.Program) *EnhancedBridge {
	return &EnhancedBridge{program: p}
}

// DrainChannel reads from an engine event channel and forwards events.
func (b *EnhancedBridge) DrainChannel(ch <-chan *engine.StreamEvent) {
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
		case engine.EventToolUse:
			inputStr := ""
			if ev.ToolInput != nil {
				if data, err := json.Marshal(ev.ToolInput); err == nil {
					inputStr = string(data)
				}
			}
			b.program.Send(tui.ToolStartMsg{
				ToolID:   ev.ToolID,
				ToolName: ev.ToolName,
				Input:    inputStr,
			})
		case engine.EventToolResult:
			b.program.Send(tui.ToolDoneMsg{
				ToolID:  ev.ToolID,
				Output:  ev.Text,
				IsError: ev.IsError,
			})
		case engine.EventUsage:
			if ev.Usage != nil {
				b.program.Send(tui.CostUpdateMsg{
					InputTokens: ev.Usage.InputTokens,
				})
			}
		}
	}
	b.program.Send(tui.StreamDoneMsg{})
}

// SendPermissionRequest sends a permission request to the TUI and blocks
// until the user responds.
func (b *EnhancedBridge) SendPermissionRequest(toolName, desc string) bool {
	ch := make(chan bool, 1)
	b.program.Send(PermissionRequestMsg{
		ToolName:    toolName,
		Description: desc,
		ResponseCh:  ch,
	})
	return <-ch
}
