package message

import (
	"fmt"
	"strings"
)

// GroupType identifies the kind of message group.
type GroupType string

const (
	GroupSingle      GroupType = "single"
	GroupToolBatch   GroupType = "tool_batch"
	GroupReadSearch  GroupType = "read_search"
	GroupHookSummary GroupType = "hook_summary"
)

// MessageGroup is a logical grouping of messages for display.
type MessageGroup struct {
	Type      GroupType
	Messages  []RenderableMessage
	Collapsed bool
	Label     string // e.g. "3 file reads" for collapsed groups
}

// readSearchTools lists tools that can be collapsed into a single group.
var readSearchTools = map[string]bool{
	"Read": true, "read": true,
	"Grep": true, "grep": true,
	"Glob": true, "glob": true,
	"WebSearch": true, "web_search": true,
}

// GroupMessages applies grouping logic to a flat message list.
// Adjacent read/search tool calls are collapsed into a single group.
// Adjacent tool_use + tool_result pairs are grouped.
func GroupMessages(msgs []RenderableMessage) []MessageGroup {
	var groups []MessageGroup
	i := 0

	for i < len(msgs) {
		msg := msgs[i]

		// Try to form a read/search group
		if msg.Type == TypeToolUse && readSearchTools[msg.ToolName] {
			group := MessageGroup{
				Type:     GroupReadSearch,
				Messages: []RenderableMessage{msg},
			}
			j := i + 1
			// Consume the tool result
			if j < len(msgs) && msgs[j].Type == TypeToolResult {
				group.Messages = append(group.Messages, msgs[j])
				j++
			}
			// Continue consuming adjacent read/search pairs
			for j < len(msgs) && msgs[j].Type == TypeToolUse && readSearchTools[msgs[j].ToolName] {
				group.Messages = append(group.Messages, msgs[j])
				j++
				if j < len(msgs) && msgs[j].Type == TypeToolResult {
					group.Messages = append(group.Messages, msgs[j])
					j++
				}
			}
			// Only group if more than one pair
			if len(group.Messages) > 2 {
				count := 0
				for _, m := range group.Messages {
					if m.Type == TypeToolUse {
						count++
					}
				}
				group.Label = formatReadSearchGroupLabel(group.Messages)
				group.Collapsed = true
				groups = append(groups, group)
			} else {
				// Not enough to group — emit individually
				for _, m := range group.Messages {
					groups = append(groups, MessageGroup{
						Type:     GroupSingle,
						Messages: []RenderableMessage{m},
					})
				}
			}
			i = j
			continue
		}

		// Try to form a tool batch (tool_use + tool_result pair)
		if msg.Type == TypeToolUse && i+1 < len(msgs) && msgs[i+1].Type == TypeToolResult {
			groups = append(groups, MessageGroup{
				Type:     GroupToolBatch,
				Messages: []RenderableMessage{msg, msgs[i+1]},
			})
			i += 2
			continue
		}

		// Single message
		groups = append(groups, MessageGroup{
			Type:     GroupSingle,
			Messages: []RenderableMessage{msg},
		})
		i++
	}

	return groups
}

// CollapseReadSearchGroups marks read/search groups as collapsed.
func CollapseReadSearchGroups(groups []MessageGroup) []MessageGroup {
	for i := range groups {
		if groups[i].Type == GroupReadSearch {
			groups[i].Collapsed = true
		}
	}
	return groups
}

// ExpandGroup marks a group as expanded.
func ExpandGroup(groups []MessageGroup, idx int) []MessageGroup {
	if idx >= 0 && idx < len(groups) {
		groups[idx].Collapsed = false
	}
	return groups
}

func formatGroupLabel(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

// formatReadSearchGroupLabel generates a label like "Read 3 files, Grep 2 patterns"
// matching claude-code-main's collapsible read/search group labels.
func formatReadSearchGroupLabel(msgs []RenderableMessage) string {
	counts := make(map[string]int)
	for _, m := range msgs {
		if m.Type == TypeToolUse {
			counts[m.ToolName]++
		}
	}

	var parts []string
	for name, count := range counts {
		switch name {
		case "Read", "read":
			parts = append(parts, formatGroupLabel(count, "file read"))
		case "Grep", "grep":
			parts = append(parts, formatGroupLabel(count, "grep search"))
		case "Glob", "glob":
			parts = append(parts, formatGroupLabel(count, "glob pattern"))
		case "WebSearch", "web_search":
			parts = append(parts, formatGroupLabel(count, "web search"))
		default:
			parts = append(parts, formatGroupLabel(count, name))
		}
	}
	return strings.Join(parts, ", ")
}

// RenderCollapsedGroup renders a collapsed group as a single summary line:
//
//	● Read 3 files, Grep 2 patterns
func RenderCollapsedGroup(group MessageGroup, opts RenderOpts) string {
	s := opts.styles()
	if group.Label == "" {
		group.Label = "collapsed operations"
	}
	return s.Dim.Render("● " + group.Label)
}

// RenderMessageGroupRow renders a message group.
// If collapsed, renders a single summary line.
// If expanded, renders each message individually.
func RenderMessageGroupRow(group MessageGroup, opts RenderOpts) string {
	if group.Collapsed {
		return RenderCollapsedGroup(group, opts)
	}

	var lines []string
	for _, msg := range group.Messages {
		lines = append(lines, RenderMessageRow(msg, opts))
	}
	return strings.Join(lines, "\n")
}
