package tui

import (
	"fmt"

	"github.com/wall-ai/agent-engine/internal/command"
)

// CommandCompletionItems converts command.CompletionEntry values into TUI
// CompletionItem values. This is the bridge between the command registry
// and the TUI's Completer.
func CommandCompletionItems(entries []command.CompletionEntry) []CompletionItem {
	items := make([]CompletionItem, 0, len(entries))
	for _, e := range entries {
		label := "/" + e.Name
		desc := e.Description
		if e.ArgHint != "" {
			desc = fmt.Sprintf("%s — %s", e.ArgHint, desc)
		}
		items = append(items, CompletionItem{
			Label:       label,
			Value:       label,
			Description: desc,
			Kind:        CompletionCommand,
		})
	}
	return items
}

// RefreshCompleterFromRegistry updates a Completer's command list from
// a command.Registry. Call this after dynamic commands are loaded or
// when the session context changes.
func RefreshCompleterFromRegistry(c *Completer, r *command.Registry, ectx *command.ExecContext) {
	entries := command.CompletionEntries(r, ectx)
	items := CommandCompletionItems(entries)
	c.SetCommands(items)
}
