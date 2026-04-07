package toolsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Query string `json:"query"`
}

// ScoredResult is a search hit with a relevance score.
type ScoredResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Score       int    `json:"score"`
}

// ToolSearchTool performs lazy tool discovery — given a natural-language query
// it returns matching tool names and descriptions from the registry.
// Aligned with claude-code-main's ToolSearchTool: supports select: syntax,
// multi-dimensional keyword scoring, and deferred tool filtering.
type ToolSearchTool struct {
	tool.BaseTool
	registry toolLister
}

type toolLister interface {
	All() []engine.Tool
}

func New(registry toolLister) *ToolSearchTool {
	return &ToolSearchTool{registry: registry}
}

func (t *ToolSearchTool) Name() string           { return "ToolSearch" }
func (t *ToolSearchTool) UserFacingName() string { return "tool_search" }
func (t *ToolSearchTool) Description() string {
	return "Search for available tools by name or description keyword."
}
func (t *ToolSearchTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *ToolSearchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *ToolSearchTool) MaxResultSizeChars() int                  { return 16_000 }
func (t *ToolSearchTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *ToolSearchTool) IsSearchOrRead(_ json.RawMessage) engine.SearchOrReadInfo {
	return engine.SearchOrReadInfo{IsSearch: true}
}
func (t *ToolSearchTool) AlwaysLoad() bool { return true }

func (t *ToolSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"query":{"type":"string","description":"Keyword or phrase to search tool names and descriptions. Use 'select:<tool_name>' to directly select a known tool."}
		},
		"required":["query"]
	}`)
}

func (t *ToolSearchTool) Prompt(_ *tool.UseContext) string {
	return `Search for available tools by name or description keyword. Use this tool to discover deferred tools that are not loaded by default.

Usage:
- Use "select:<tool_name>" for direct selection of a known tool
- Use keywords to search tool names and descriptions
- Results are ranked by relevance (name match > description match > hint match)
- Returns matching tool names and descriptions`
}

func (t *ToolSearchTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *ToolSearchTool) Call(_ context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	_ = json.Unmarshal(input, &in)

	ch := make(chan *engine.ContentBlock, 1)
	go func() {
		defer close(ch)
		query := strings.TrimSpace(in.Query)

		if t.registry == nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "[]"}
			return
		}

		allTools := t.registry.All()

		// Handle select: syntax — direct tool selection by name.
		if strings.HasPrefix(strings.ToLower(query), "select:") {
			toolName := strings.TrimSpace(query[7:])
			for _, tl := range allTools {
				if strings.EqualFold(tl.Name(), toolName) {
					out, _ := json.MarshalIndent([]ScoredResult{{
						Name:        tl.Name(),
						Description: tl.Description(),
						Score:       10,
					}}, "", "  ")
					ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(out)}
					return
				}
				// Check aliases.
				for _, alias := range tl.Aliases() {
					if strings.EqualFold(alias, toolName) {
						out, _ := json.MarshalIndent([]ScoredResult{{
							Name:        tl.Name(),
							Description: tl.Description(),
							Score:       10,
						}}, "", "  ")
						ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(out)}
						return
					}
				}
			}
			// Not found.
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: fmt.Sprintf("No tool found with name %q", toolName)}
			return
		}

		// Keyword scoring — aligned with claude-code-main.
		queryLower := strings.ToLower(query)
		queryWords := strings.Fields(queryLower)

		var results []ScoredResult
		for _, tl := range allTools {
			score := scoreTool(tl, queryLower, queryWords)
			if score > 0 {
				results = append(results, ScoredResult{
					Name:        tl.Name(),
					Description: tl.Description(),
					Score:       score,
				})
			}
		}

		// Sort by score descending.
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})

		// Cap at 20 results.
		if len(results) > 20 {
			results = results[:20]
		}

		out, _ := json.MarshalIndent(results, "", "  ")
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(out)}
	}()
	return ch, nil
}

// MapToolResultToBlockParam formats the search result for the model.
func (t *ToolSearchTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text, ok := content.(string)
	if !ok {
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: ""}
	}

	// Parse results and format for the model.
	var results []ScoredResult
	if json.Unmarshal([]byte(text), &results) == nil && len(results) > 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d matching tools:\n\n", len(results)))
		for _, r := range results {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", r.Name, r.Description))
		}
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: sb.String()}
	}

	return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: text}
}

// scoreTool computes a relevance score (0–10) for a tool against the query.
// Scoring dimensions (aligned with claude-code-main):
//   - Exact name match: 10
//   - Name contains query: 7
//   - Alias exact match: 8
//   - Name word overlap: 5
//   - Description contains query: 3
//   - SearchHint contains query: 2
//   - Individual word matches in name/description: 1 per word
func scoreTool(tl engine.Tool, queryLower string, queryWords []string) int {
	nameLower := strings.ToLower(tl.Name())
	descLower := strings.ToLower(tl.Description())
	hintLower := strings.ToLower(tl.SearchHint())

	// Exact name match.
	if nameLower == queryLower {
		return 10
	}

	// Alias exact match.
	for _, alias := range tl.Aliases() {
		if strings.ToLower(alias) == queryLower {
			return 8
		}
	}

	score := 0

	// Name contains full query.
	if strings.Contains(nameLower, queryLower) {
		score = max(score, 7)
	}

	// Description contains full query.
	if strings.Contains(descLower, queryLower) {
		score = max(score, 3)
	}

	// SearchHint contains full query.
	if hintLower != "" && strings.Contains(hintLower, queryLower) {
		score = max(score, 2)
	}

	// Per-word matching.
	nameWords := strings.Fields(nameLower)
	for _, qw := range queryWords {
		for _, nw := range nameWords {
			if strings.Contains(nw, qw) {
				score = max(score, 5)
			}
		}
		if strings.Contains(descLower, qw) {
			score = max(score, 1)
		}
		if hintLower != "" && strings.Contains(hintLower, qw) {
			score = max(score, 1)
		}
	}

	return score
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
