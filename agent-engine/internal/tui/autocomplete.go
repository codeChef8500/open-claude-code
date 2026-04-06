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

// SetCommands replaces the current command completion list.
func (c *Completer) SetCommands(cmds []CompletionItem) {
	c.commands = cmds
}

// SetTools replaces the current tool completion list.
func (c *Completer) SetTools(tools []CompletionItem) {
	c.tools = tools
}

// DefaultSlashCommands returns the built-in slash commands for autocomplete.
// This is the static fallback list; use RefreshCompleterFromRegistry (in
// command_bridge.go) for a dynamic list that tracks the command registry.
func DefaultSlashCommands() []CompletionItem {
	type entry struct{ name, desc string }
	cmds := []entry{
		// Core
		{"/help", "Show available commands"},
		{"/compact", "Compact conversation history"},
		{"/clear", "Clear conversation history and free up context"},
		{"/context", "Show context window usage"},
		{"/cost", "Show token usage and cost"},
		{"/model", "Switch or show current model"},
		{"/status", "Show session status"},
		{"/version", "Show version information"},
		{"/quit", "Exit the current session"},

		// Config & settings
		{"/config", "Open config panel"},
		{"/mcp", "Manage MCP servers"},
		{"/permissions", "Manage allow & deny tool permission rules"},
		{"/hooks", "View hook configurations for tool events"},
		{"/memory", "Edit Claude memory files"},
		{"/theme", "Change the theme"},
		{"/color", "Change color scheme"},
		{"/vim", "Toggle vim keybinding mode"},
		{"/verbose", "Toggle verbose output"},
		{"/privacy-settings", "View or change privacy settings"},
		{"/sandbox-toggle", "Toggle sandbox mode"},
		{"/terminal-setup", "Configure terminal integration"},

		// Session & conversation
		{"/plan", "Toggle plan mode or view session plan"},
		{"/fast", "Toggle fast mode (smaller, faster model)"},
		{"/effort", "Set effort level for model usage"},
		{"/auto-mode", "Toggle or show Auto Mode status"},
		{"/session", "Show session info and remote URL"},
		{"/resume", "Resume a previous conversation"},
		{"/rename", "Rename the current conversation"},
		{"/branch", "Create a branch of the current conversation"},
		{"/rewind", "Restore code and/or conversation to a previous point"},
		{"/export", "Export conversation to a file or clipboard"},
		{"/copy", "Copy last response to clipboard"},

		// Git & code
		{"/diff", "Show uncommitted changes and per-turn diffs"},
		{"/commit", "Create a git commit with generated message"},
		{"/review", "Review recent changes"},
		{"/security-review", "Run a security-focused review"},
		{"/pr-comments", "Address PR review comments"},
		{"/init", "Initialize project configuration"},

		// Agents & tasks
		{"/agents", "Manage agent configurations"},
		{"/tasks", "List and manage background tasks"},
		{"/workflow", "Manage and run workflows"},

		// Account & auth
		{"/login", "Sign in with your Anthropic account"},
		{"/logout", "Sign out from your Anthropic account"},
		{"/usage", "Show plan usage limits"},

		// System
		{"/bug-report", "Report a bug with diagnostic info"},
		{"/doctor", "Diagnose and verify installation"},
		{"/skills", "List available skills"},
		{"/plugin", "Manage plugins"},
		{"/feedback", "Send feedback about openclaude-go"},
		{"/stats", "Show session statistics"},
		{"/upgrade", "Upgrade to the latest version"},
		{"/add-dir", "Add a new working directory"},
		{"/desktop", "Desktop app integration info"},
		{"/mobile", "Mobile app integration info"},

		// UI
		{"/keybindings", "Show or configure keyboard shortcuts"},
		{"/stickers", "Order openclaude-go stickers"},
		{"/release-notes", "Show release notes"},

		// Companion
		{"/buddy", "Interact with your companion buddy"},
	}

	items := make([]CompletionItem, 0, len(cmds))
	for _, c := range cmds {
		items = append(items, CompletionItem{
			Label:       c.name,
			Value:       c.name,
			Description: c.desc,
			Kind:        CompletionCommand,
		})
	}
	return items
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
	seen := map[string]bool{}
	for _, cmd := range c.commands {
		if strings.HasPrefix(strings.ToLower(cmd.Label), prefix) {
			if !seen[cmd.Label] {
				matches = append(matches, cmd)
				seen[cmd.Label] = true
			}
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Label < matches[j].Label
	})
	// Cap at 15 visible suggestions.
	if len(matches) > 15 {
		matches = matches[:15]
	}
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
