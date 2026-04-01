package prompt

import "fmt"

const (
	// TotalToolResultBudget is the maximum total characters allowed across all
	// tool results in a single turn before truncation kicks in.
	TotalToolResultBudget = 100_000

	// SingleResultBudget is the maximum characters for any one tool result.
	SingleResultBudget = 25_000
)

// TruncateToolResult truncates a single tool result string to SingleResultBudget,
// appending a descriptive suffix so the model understands content was omitted.
func TruncateToolResult(result string) string {
	if len(result) <= SingleResultBudget {
		return result
	}
	removed := len(result) - SingleResultBudget
	return result[:SingleResultBudget] + fmt.Sprintf("\n\n[... %d characters truncated — output exceeded single-result budget ...]", removed)
}

// ApplyToolResultBudget iterates a slice of (toolName, content) pairs,
// truncating each to SingleResultBudget and stopping early once the cumulative
// total reaches TotalToolResultBudget.  It returns a new slice with the
// applied budget limits.
func ApplyToolResultBudget(results []ToolResultEntry) []ToolResultEntry {
	out := make([]ToolResultEntry, 0, len(results))
	total := 0

	for _, r := range results {
		if total >= TotalToolResultBudget {
			out = append(out, ToolResultEntry{
				ToolName: r.ToolName,
				Content:  "[tool result omitted — total turn budget exhausted]",
			})
			continue
		}

		content := TruncateToolResult(r.Content)
		remaining := TotalToolResultBudget - total
		if len(content) > remaining {
			removed := len(content) - remaining
			content = content[:remaining] + fmt.Sprintf("\n\n[... %d characters truncated — total turn budget exceeded ...]", removed)
		}

		total += len(content)
		out = append(out, ToolResultEntry{
			ToolName: r.ToolName,
			Content:  content,
		})
	}

	return out
}

// ToolResultEntry pairs a tool name with its raw result string for budget
// processing.
type ToolResultEntry struct {
	ToolName string
	Content  string
}
