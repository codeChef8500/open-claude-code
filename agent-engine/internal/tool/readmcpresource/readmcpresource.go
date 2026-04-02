package readmcpresource

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/service/mcp"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// ReadMcpResourceTool reads a specific resource from an MCP server.
type ReadMcpResourceTool struct {
	tool.BaseTool
	manager *mcp.Manager
}

// New creates a new ReadMcpResourceTool.
func New(manager *mcp.Manager) *ReadMcpResourceTool {
	return &ReadMcpResourceTool{manager: manager}
}

func (t *ReadMcpResourceTool) Name() string           { return "read_mcp_resource" }
func (t *ReadMcpResourceTool) UserFacingName() string  { return "ReadMcpResource" }
func (t *ReadMcpResourceTool) Description() string {
	return "Read a specific resource from an MCP server by server name and URI."
}
func (t *ReadMcpResourceTool) MaxResultSizeChars() int                   { return 100_000 }
func (t *ReadMcpResourceTool) IsEnabled(_ *tool.UseContext) bool         { return true }
func (t *ReadMcpResourceTool) IsReadOnly(_ json.RawMessage) bool         { return true }
func (t *ReadMcpResourceTool) IsConcurrencySafe(_ json.RawMessage) bool  { return true }
func (t *ReadMcpResourceTool) ShouldDefer() bool                         { return true }
func (t *ReadMcpResourceTool) SearchHint() string                        { return "read a specific MCP resource by URI" }

func (t *ReadMcpResourceTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"server": {
				"type": "string",
				"description": "The MCP server name."
			},
			"uri": {
				"type": "string",
				"description": "The resource URI to read."
			}
		},
		"required": ["server", "uri"]
	}`)
}

func (t *ReadMcpResourceTool) Prompt(_ *tool.UseContext) string {
	return "Reads a specific resource from an MCP server. Use list_mcp_resources first to discover available resources."
}

func (t *ReadMcpResourceTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *ReadMcpResourceTool) Call(ctx context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 4)

	go func() {
		defer close(ch)

		var args struct {
			Server string `json:"server"`
			URI    string `json:"uri"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "invalid input: " + err.Error(), IsError: true}
			return
		}
		if args.Server == "" || args.URI == "" {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "server and uri are required", IsError: true}
			return
		}

		// Get the client.
		client, ok := t.manager.GetClient(args.Server)
		if !ok {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Server %q not found.", args.Server),
				IsError: true,
			}
			return
		}

		if client.ConnectionState() != mcp.StateConnected {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Server %q is not connected (state: %s).", args.Server, client.ConnectionState()),
				IsError: true,
			}
			return
		}

		caps := client.Capabilities()
		if caps.Resources == nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Server %q does not support resources.", args.Server),
				IsError: true,
			}
			return
		}

		result, err := client.ReadResource(ctx, args.URI)
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Error reading resource %q from %q: %v", args.URI, args.Server, err),
				IsError: true,
			}
			return
		}

		if len(result.Contents) == 0 {
			ch <- &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: fmt.Sprintf("Resource %q returned empty content.", args.URI),
			}
			return
		}

		// Convert resource contents to content blocks.
		for _, item := range result.Contents {
			switch {
			case item.Text != "":
				text := item.Text
				if len(text) > mcp.MaxMCPToolResultChars {
					text = text[:mcp.MaxMCPToolResultChars] + "\n... [truncated]"
				}
				ch <- &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: text,
				}
			case item.Data != "":
				// Binary/blob content — report as saved or truncate base64.
				ch <- &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: fmt.Sprintf("[Binary content from %s, MIME: %s, %d bytes encoded]",
						args.URI, item.MimeType, len(item.Data)),
				}
			default:
				data, _ := json.Marshal(item)
				ch <- &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: string(data),
				}
			}
		}
	}()

	return ch, nil
}
