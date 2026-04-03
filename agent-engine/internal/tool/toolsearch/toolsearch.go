package toolsearch

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Query string `json:"query"`
}

// ToolSearchTool performs lazy tool discovery — given a natural-language query
// it returns matching tool names and descriptions from the registry.
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
			"query":{"type":"string","description":"Keyword or phrase to search tool names and descriptions."}
		},
		"required":["query"]
	}`)
}

func (t *ToolSearchTool) Prompt(_ *tool.UseContext) string {
	return `Search for available tools by name or description keyword. Use this tool to discover deferred tools that are not loaded by default.

Usage:
- Use "select:<tool_name>" for direct selection of a known tool
- Use keywords to search tool names and descriptions
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
		query := strings.ToLower(strings.TrimSpace(in.Query))

		type result struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		var results []result

		if t.registry != nil {
			for _, tl := range t.registry.All() {
				name := strings.ToLower(tl.Name())
				desc := strings.ToLower(tl.Description())
				if query == "" || strings.Contains(name, query) || strings.Contains(desc, query) {
					results = append(results, result{
						Name:        tl.Name(),
						Description: tl.Description(),
					})
				}
			}
		}

		out, _ := json.MarshalIndent(results, "", "  ")
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(out)}
	}()
	return ch, nil
}
