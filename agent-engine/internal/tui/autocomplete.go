package tui

import (
	"path/filepath"
	"sort"
	"strings"
)

// ────────────────────────────────────────────────────────────────────────────
// Autocomplete — provides slash command and @-mention completion for the TUI.
// Aligned with claude-code-main's autocomplete patterns.
// ────────────────────────────────────────────────────────────────────────────

// CompletionItem is a single autocomplete suggestion.
type CompletionItem struct {
	// Label is the display text shown in the completion menu.
	Label string
	// Value is the text inserted on selection.
	Value string
	// Description is an optional explanation shown next to the label.
	Description string
	// Kind classifies the completion (command, file, tool, etc.).
	Kind CompletionKind
}

// CompletionKind classifies the type of completion.
type CompletionKind string

const (
	CompletionCommand CompletionKind = "command"
	CompletionFile    CompletionKind = "file"
	CompletionTool    CompletionKind = "tool"
	CompletionFlag    CompletionKind = "flag"
)

// Completer provides autocomplete suggestions for the TUI input.
type Completer struct {
	// commands are the registered slash commands.
	commands []CompletionItem
	// tools are the available tool names.
	tools []CompletionItem
	// recentFiles are recently referenced file paths.
	recentFiles []string
}

// NewCompleter creates a completer with the given commands and tools.
func NewCompleter(commands []CompletionItem, tools []CompletionItem) *Completer {
	return &Completer{
		commands: commands,
		tools:    tools,
	}
}

// DefaultSlashCommands returns the built-in slash commands.
func DefaultSlashCommands() []CompletionItem {
	return []CompletionItem{
		{Label: "/help", Value: "/help", Description: "Show available commands", Kind: CompletionCommand},
		{Label: "/compact", Value: "/compact", Description: "Compact conversation history", Kind: CompletionCommand},
		{Label: "/clear", Value: "/clear", Description: "Clear conversation", Kind: CompletionCommand},
		{Label: "/context", Value: "/context", Description: "Show context usage", Kind: CompletionCommand},
		{Label: "/cost", Value: "/cost", Description: "Show token costs", Kind: CompletionCommand},
		{Label: "/config", Value: "/config", Description: "Show/set configuration", Kind: CompletionCommand},
		{Label: "/memory", Value: "/memory", Description: "Manage session memory", Kind: CompletionCommand},
		{Label: "/model", Value: "/model", Description: "Switch model", Kind: CompletionCommand},
		{Label: "/permissions", Value: "/permissions", Description: "Show permission settings", Kind: CompletionCommand},
		{Label: "/status", Value: "/status", Description: "Show session status", Kind: CompletionCommand},
		{Label: "/bug", Value: "/bug", Description: "Report a bug", Kind: CompletionCommand},
		{Label: "/quit", Value: "/quit", Description: "Exit the application", Kind: CompletionCommand},
		{Label: "/vim", Value: "/vim", Description: "Toggle vim mode", Kind: CompletionCommand},
		{Label: "/login", Value: "/login", Description: "Authenticate", Kind: CompletionCommand},
		{Label: "/doctor", Value: "/doctor", Description: "Check system health", Kind: CompletionCommand},
		{Label: "/init", Value: "/init", Description: "Initialize CLAUDE.md", Kind: CompletionCommand},
		{Label: "/mcp", Value: "/mcp", Description: "MCP server management", Kind: CompletionCommand},
		{Label: "/review", Value: "/review", Description: "Review recent changes", Kind: CompletionCommand},
	}
}

// AddRecentFile adds a file path to the recent files list for @-mention completion.
func (c *Completer) AddRecentFile(path string) {
	// Deduplicate.
	for _, f := range c.recentFiles {
		if f == path {
			return
		}
	}
	c.recentFiles = append(c.recentFiles, path)
	// Cap at 50 recent files.
	if len(c.recentFiles) > 50 {
		c.recentFiles = c.recentFiles[len(c.recentFiles)-50:]
	}
}

// Complete returns suggestions for the given input text and cursor position.
func (c *Completer) Complete(input string, cursorPos int) []CompletionItem {
	if cursorPos > len(input) {
		cursorPos = len(input)
	}
	prefix := input[:cursorPos]

	// Slash commands: triggered at start of input.
	if strings.HasPrefix(prefix, "/") {
		return c.completeSlashCommand(prefix)
	}

	// @-mention: triggered by @ followed by partial path.
	atIdx := strings.LastIndex(prefix, "@")
	if atIdx >= 0 {
		partial := prefix[atIdx+1:]
		return c.completeAtMention(partial)
	}

	return nil
}

func (c *Completer) completeSlashCommand(prefix string) []CompletionItem {
	prefix = strings.ToLower(prefix)
	var matches []CompletionItem
	for _, cmd := range c.commands {
		if strings.HasPrefix(strings.ToLower(cmd.Label), prefix) {
			matches = append(matches, cmd)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Label < matches[j].Label
	})
	return matches
}

func (c *Completer) completeAtMention(partial string) []CompletionItem {
	partial = strings.ToLower(partial)
	var matches []CompletionItem

	for _, f := range c.recentFiles {
		base := filepath.Base(f)
		if strings.HasPrefix(strings.ToLower(base), partial) ||
			strings.HasPrefix(strings.ToLower(f), partial) {
			matches = append(matches, CompletionItem{
				Label:       "@" + f,
				Value:       "@" + f,
				Description: "file",
				Kind:        CompletionFile,
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Label < matches[j].Label
	})

	// Limit to 10 suggestions.
	if len(matches) > 10 {
		matches = matches[:10]
	}
	return matches
}

// CompletionState tracks the current autocomplete UI state.
type CompletionState struct {
	// Active indicates whether the completion menu is visible.
	Active bool
	// Items are the current suggestions.
	Items []CompletionItem
	// Selected is the index of the highlighted item.
	Selected int
	// Prefix is the text that triggered the completion.
	Prefix string
}

// Reset clears the completion state.
func (cs *CompletionState) Reset() {
	cs.Active = false
	cs.Items = nil
	cs.Selected = 0
	cs.Prefix = ""
}

// SelectNext moves the selection down.
func (cs *CompletionState) SelectNext() {
	if len(cs.Items) == 0 {
		return
	}
	cs.Selected = (cs.Selected + 1) % len(cs.Items)
}

// SelectPrev moves the selection up.
func (cs *CompletionState) SelectPrev() {
	if len(cs.Items) == 0 {
		return
	}
	cs.Selected--
	if cs.Selected < 0 {
		cs.Selected = len(cs.Items) - 1
	}
}

// SelectedItem returns the currently selected item, or nil if none.
func (cs *CompletionState) SelectedItem() *CompletionItem {
	if !cs.Active || len(cs.Items) == 0 {
		return nil
	}
	return &cs.Items[cs.Selected]
}
