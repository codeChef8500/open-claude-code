package toolui

import (
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/tui/figures"
)

// CollapsedItem represents a single tool call within a collapsed group.
type CollapsedItem struct {
	FilePath string
	ToolName string // "Read", "Search", "Grep"
}

// CollapsedGroup renders consecutive read/search tool calls collapsed into
// a single summary line, matching claude-code-main's CollapsedReadSearchContent.
//
// Layout:
//
//	● Read (5 files)          — grey dot, active blinks
//	● Search (3 patterns)     — expanded shows details
type CollapsedGroup struct {
	ToolLabel string // "Read" or "Search"
	Items     []CollapsedItem
	Active    bool // true if any tool in the group is still running
	Expanded  bool // true if user toggled expand (Ctrl+O)
	theme     ToolUITheme
}

// NewCollapsedGroup creates a new collapsed group.
func NewCollapsedGroup(toolLabel string, theme ToolUITheme) *CollapsedGroup {
	return &CollapsedGroup{
		ToolLabel: toolLabel,
		theme:     theme,
	}
}

// Add appends an item to the group.
func (g *CollapsedGroup) Add(item CollapsedItem) {
	g.Items = append(g.Items, item)
}

// Count returns the number of items in the group.
func (g *CollapsedGroup) Count() int {
	return len(g.Items)
}

// Reset clears the group.
func (g *CollapsedGroup) Reset() {
	g.Items = nil
	g.Active = false
	g.Expanded = false
}

// View renders the collapsed group.
//
// Collapsed: ● Read (5 files)
// Expanded:
//
//	● Read (5 files)
//	  ⎿  /src/main.go
//	  │  /src/utils.go
//	  │  /src/config.go
func (g *CollapsedGroup) View(dotView string) string {
	if len(g.Items) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header: ● Read (5 files)
	itemLabel := "files"
	if g.ToolLabel == "Search" || g.ToolLabel == "Grep" {
		itemLabel = "patterns"
		if len(g.Items) == 1 {
			itemLabel = "pattern"
		}
	} else if len(g.Items) == 1 {
		itemLabel = "file"
	}

	params := fmt.Sprintf("%d %s", len(g.Items), itemLabel)
	header := RenderToolHeader(dotView, g.ToolLabel, params, g.theme)
	sb.WriteString(header)

	if !g.Expanded {
		// Show Ctrl+O hint
		sb.WriteString("  ")
		sb.WriteString(g.theme.Dim.Render("(Ctrl+O to expand)"))
		return sb.String()
	}

	// Expanded: show each file
	for i, item := range g.Items {
		sb.WriteString("\n")
		if i == 0 {
			sb.WriteString(g.theme.TreeConn.Render(ResponsePrefix))
		} else {
			sb.WriteString(g.theme.TreeConn.Render("  │  "))
		}
		display := item.FilePath
		if len(display) > 50 {
			display = "…" + display[len(display)-49:]
		}
		sb.WriteString(g.theme.FilePath.Render(display))
	}

	return sb.String()
}

// ViewWithDefaultDot renders the group using a default grey dot.
func (g *CollapsedGroup) ViewWithDefaultDot() string {
	dotStyle := g.theme.Dim
	dot := dotStyle.Render(figures.BlackCircle()) + " "
	return g.View(dot)
}
