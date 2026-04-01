package prompt

import (
	"sort"
	"strings"

	"github.com/wall-ai/agent-engine/internal/tool"
)

// BuildToolsPrompt assembles the tool descriptions section of the system prompt.
// Tools are sorted deterministically to maximise prompt cache stability.
func BuildToolsPrompt(tools []tool.Tool, uctx *tool.UseContext) string {
	if len(tools) == 0 {
		return ""
	}

	// Sort by name for stable ordering (built-in before MCP via prefix sorting).
	sorted := make([]tool.Tool, len(tools))
	copy(sorted, tools)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Name() < sorted[j].Name()
	})

	var parts []string
	for _, t := range sorted {
		if !t.IsEnabled(uctx) {
			continue
		}
		snippet := t.Prompt(uctx)
		if snippet != "" {
			parts = append(parts, snippet)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}
