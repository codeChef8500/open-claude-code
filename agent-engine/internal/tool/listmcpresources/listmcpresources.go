package listmcpresources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/service/mcp"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// ListMcpResourcesTool lists resources from connected MCP servers.
type ListMcpResourcesTool struct {
	tool.BaseTool
	manager *mcp.Manager
}

// New creates a new ListMcpResourcesTool.
func New(manager *mcp.Manager) *ListMcpResourcesTool {
	return &ListMcpResourcesTool{manager: manager}
}

func (t *ListMcpResourcesTool) Name() string           { return "list_mcp_resources" }
func (t *ListMcpResourcesTool) UserFacingName() string  { return "ListMcpResources" }
func (t *ListMcpResourcesTool) Description() string {
	return "List resources available from connected MCP servers. Optionally filter by server name."
}
func (t *ListMcpResourcesTool) MaxResultSizeChars() int { return 100_000 }
func (t *ListMcpResourcesTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *ListMcpResourcesTool) IsReadOnly(_ json.RawMessage) bool { return true }
func (t *ListMcpResourcesTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *ListMcpResourcesTool) ShouldDefer() bool { return true }
func (t *ListMcpResourcesTool) SearchHint() string { return "list resources from connected MCP servers" }

func (t *ListMcpResourcesTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"server": {
				"type": "string",
				"description": "Optional server name to filter resources by."
			}
		}
	}`)
}

func (t *ListMcpResourcesTool) Prompt(_ *tool.UseContext) string {
	return "Lists resources from connected MCP servers. Use this to discover available resources before reading them."
}

func (t *ListMcpResourcesTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *ListMcpResourcesTool) Call(_ context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)

	go func() {
		defer close(ch)

		var args struct {
			Server string `json:"server"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "invalid input: " + err.Error(), IsError: true}
			return
		}

		allResources := t.manager.AllResources()

		// Filter by server if specified.
		if args.Server != "" {
			var filtered []mcp.ServerResource
			for _, r := range allResources {
				if r.Server == args.Server {
					filtered = append(filtered, r)
				}
			}
			if len(filtered) == 0 {
				// Check if the server even exists.
				_, found := t.manager.GetConnectionInfo(args.Server)
				if !found {
					// List available servers.
					states := t.manager.ConnectionStates()
					var names []string
					for _, s := range states {
						names = append(names, s.Name)
					}
					ch <- &engine.ContentBlock{
						Type:    engine.ContentTypeText,
						Text:    fmt.Sprintf("Server %q not found. Available servers: %s", args.Server, strings.Join(names, ", ")),
						IsError: true,
					}
					return
				}
			}
			allResources = filtered
		}

		if len(allResources) == 0 {
			ch <- &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: "No resources found. MCP servers may still provide tools even if they have no resources.",
			}
			return
		}

		// Build output.
		type resourceInfo struct {
			URI         string `json:"uri"`
			Name        string `json:"name"`
			MimeType    string `json:"mimeType,omitempty"`
			Description string `json:"description,omitempty"`
			Server      string `json:"server"`
		}

		var items []resourceInfo
		for _, r := range allResources {
			items = append(items, resourceInfo{
				URI:         r.URI,
				Name:        r.Name,
				MimeType:    r.MimeType,
				Description: r.Description,
				Server:      r.Server,
			})
		}

		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "marshal error: " + err.Error(), IsError: true}
			return
		}

		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: string(data),
		}
	}()

	return ch, nil
}
