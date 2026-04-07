package toolui

import (
	"fmt"
	"strings"
	"time"
)

// TodoItemDisplay is the minimal data needed to render a todo item.
type TodoItemDisplay struct {
	Content    string
	Status     string // "pending" | "in_progress" | "completed"
	ActiveForm string
}

// TodoToolUI renders todo list tool use.
// Layout matches claude-code-main's TodoWriteTool + TaskListV2 component.
//
//	(no header — TodoWrite renders invisible in claude-code)
//	  ☐ Pending task
//	  ◉ In progress task
//	  ✓ Completed task
type TodoToolUI struct {
	theme ToolUITheme
}

// NewTodoToolUI creates a todo tool renderer.
func NewTodoToolUI(theme ToolUITheme) *TodoToolUI {
	return &TodoToolUI{theme: theme}
}

// statusIcon returns the icon for a given todo status.
func statusIcon(status string) string {
	switch status {
	case "completed":
		return "✓"
	case "in_progress":
		return "◉"
	default:
		return "☐"
	}
}

// RenderTaskList renders the full task list for display.
// This can be shown in the status area or as a standalone view.
//
//	☐ Create dark mode toggle
//	◉ Adding state management
//	✓ Implement CSS styles
func (t *TodoToolUI) RenderTaskList(items []TodoItemDisplay, width int) string {
	if len(items) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, item := range items {
		if i > 0 {
			sb.WriteString("\n")
		}
		icon := statusIcon(item.Status)
		var style func(string) string

		switch item.Status {
		case "completed":
			style = func(s string) string { return t.theme.Dim.Render(s) }
		case "in_progress":
			style = func(s string) string { return t.theme.ToolIcon.Bold(true).Render(s) }
		default:
			style = func(s string) string { return t.theme.Output.Render(s) }
		}

		content := item.Content
		if len(content) > width-6 && width > 10 {
			content = content[:width-9] + "…"
		}
		sb.WriteString(style(fmt.Sprintf("  %s %s", icon, content)))
	}
	return sb.String()
}

// RenderCompact renders a compact one-line summary of the task list.
//
//	Tasks: 2/5 done, working on "Adding state management"
func (t *TodoToolUI) RenderCompact(items []TodoItemDisplay) string {
	if len(items) == 0 {
		return ""
	}

	completed := 0
	var current string
	for _, item := range items {
		if item.Status == "completed" {
			completed++
		}
		if item.Status == "in_progress" {
			if item.ActiveForm != "" {
				current = item.ActiveForm
			} else {
				current = item.Content
			}
		}
	}

	summary := fmt.Sprintf("Tasks: %d/%d done", completed, len(items))
	if current != "" {
		summary += fmt.Sprintf(", working on %q", current)
	}
	return t.theme.Dim.Render(summary)
}

// GetActiveForm returns the activeForm of the current in_progress todo,
// or empty string if none. Used by the spinner to override verb display.
func GetActiveForm(items []TodoItemDisplay) string {
	for _, item := range items {
		if item.Status == "in_progress" {
			if item.ActiveForm != "" {
				return item.ActiveForm
			}
			return item.Content
		}
	}
	return ""
}

// RenderTaskListWithHeader renders the task list with a progress bar header,
// matching claude-code-main's TaskListV2 component.
//
//	Tasks [████████░░░░] 3/5
//	  ✓ Implement CSS styles
//	  ◉ Adding state management
//	  ☐ Create dark mode toggle
func (t *TodoToolUI) RenderTaskListWithHeader(items []TodoItemDisplay, width int) string {
	if len(items) == 0 {
		return ""
	}

	completed := 0
	for _, item := range items {
		if item.Status == "completed" {
			completed++
		}
	}

	// Progress bar
	barWidth := 12
	if width > 60 {
		barWidth = 20
	}
	filled := 0
	if len(items) > 0 {
		filled = (completed * barWidth) / len(items)
	}
	empty := barWidth - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	header := fmt.Sprintf("Tasks [%s] %d/%d", bar, completed, len(items))

	var sb strings.Builder
	sb.WriteString(t.theme.ToolIcon.Bold(true).Render(header))
	sb.WriteString("\n")
	sb.WriteString(t.RenderTaskList(items, width))

	return sb.String()
}

// CompletedItemExpiry is how long completed items remain visible before fading.
const CompletedItemExpiry = 60 * time.Second

// TodoItemDisplayWithTime adds completion tracking for TTL-based visibility.
type TodoItemDisplayWithTime struct {
	TodoItemDisplay
	CompletedAt time.Time
}

// FilterExpiredCompleted removes completed items older than TTL,
// matching claude-code-main's TaskListV2 expiry behavior.
func FilterExpiredCompleted(items []TodoItemDisplayWithTime) []TodoItemDisplay {
	now := time.Now()
	var result []TodoItemDisplay
	for _, item := range items {
		if item.Status == "completed" && !item.CompletedAt.IsZero() {
			if now.Sub(item.CompletedAt) > CompletedItemExpiry {
				continue // expired
			}
		}
		result = append(result, item.TodoItemDisplay)
	}
	return result
}
