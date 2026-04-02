package mcptool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/service/mcp"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// MCPTool is a dynamic tool wrapper that routes calls to an MCP server.
// Each instance represents one tool exposed by a connected MCP server.
type MCPTool struct {
	tool.BaseTool

	serverName   string
	toolName     string
	qualName     string // "mcp__server__tool"
	description  string
	prompt       string
	inputSchema  json.RawMessage
	outputSchema json.RawMessage
	manager      *mcp.Manager
}

// NewMCPTool creates an MCPTool for a specific tool on a server.
func NewMCPTool(manager *mcp.Manager, serverName string, mcpTool mcp.MCPTool) *MCPTool {
	qualName := mcp.BuildMcpToolName(serverName, mcpTool.Name)

	desc := mcpTool.Description
	if len(desc) > mcp.MaxMCPDescriptionLength {
		desc = desc[:mcp.MaxMCPDescriptionLength] + "..."
	}

	return &MCPTool{
		serverName:  serverName,
		toolName:    mcpTool.Name,
		qualName:    qualName,
		description: desc,
		prompt:      fmt.Sprintf("MCP tool '%s' from server '%s'. %s", mcpTool.Name, serverName, desc),
		inputSchema: mcpTool.InputSchema,
		manager:     manager,
	}
}

func (t *MCPTool) Name() string           { return t.qualName }
func (t *MCPTool) UserFacingName() string  { return fmt.Sprintf("%s - %s (MCP)", t.serverName, t.toolName) }
func (t *MCPTool) Description() string     { return t.description }
func (t *MCPTool) MaxResultSizeChars() int { return mcp.MaxMCPToolResultChars }

func (t *MCPTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *MCPTool) InputSchema() json.RawMessage {
	if t.inputSchema != nil {
		return t.inputSchema
	}
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

func (t *MCPTool) Prompt(_ *tool.UseContext) string { return t.prompt }

func (t *MCPTool) IsMCP() bool { return true }

func (t *MCPTool) MCPInfo() *engine.MCPToolInfo {
	return &engine.MCPToolInfo{
		ServerName: t.serverName,
		ToolName:   t.toolName,
	}
}

func (t *MCPTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *MCPTool) SearchHint() string {
	return fmt.Sprintf("MCP tool %s from %s", t.toolName, t.serverName)
}

func (t *MCPTool) ToAutoClassifierInput(input json.RawMessage) string {
	return string(input)
}

func (t *MCPTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil // Permission is handled at orchestration level via hooks.
}

func (t *MCPTool) Call(ctx context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 4)

	go func() {
		defer close(ch)

		result, err := t.manager.CallTool(ctx, t.serverName, t.toolName, input)
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("MCP tool error (%s/%s): %v", t.serverName, t.toolName, err),
				IsError: true,
			}
			return
		}

		if result.IsError {
			text := contentItemsToText(result.Content)
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    text,
				IsError: true,
			}
			return
		}

		// Convert content items to content blocks.
		for _, item := range result.Content {
			switch item.Type {
			case "text":
				text := item.Text
				if len(text) > mcp.MaxMCPToolResultChars {
					text = text[:mcp.MaxMCPToolResultChars] + "\n... [truncated]"
				}
				ch <- &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: text,
				}
			case "image":
				ch <- &engine.ContentBlock{
					Type:      engine.ContentTypeImage,
					MediaType: item.MimeType,
					Data:      item.Data,
				}
			case "resource":
				// Serialize resource content as text.
				data, _ := json.Marshal(item)
				ch <- &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: string(data),
				}
			default:
				ch <- &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: fmt.Sprintf("[%s content]", item.Type),
				}
			}
		}

		// If no content items, return a success message.
		if len(result.Content) == 0 {
			ch <- &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: "Tool executed successfully (no output).",
			}
		}
	}()

	return ch, nil
}

// contentItemsToText extracts text from a list of content items.
func contentItemsToText(items []mcp.ContentItem) string {
	var parts []string
	for _, item := range items {
		if item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	if len(parts) == 0 {
		return "MCP tool returned an error with no details."
	}
	return strings.Join(parts, "\n")
}

// BuildMCPTools creates tool instances for all tools from all connected servers.
func BuildMCPTools(manager *mcp.Manager) []engine.Tool {
	allTools := manager.AllToolsCapped()
	var tools []engine.Tool
	for _, nt := range allTools {
		tools = append(tools, NewMCPTool(manager, nt.ServerName, nt.Tool))
	}
	return tools
}

// RegisterMCPTools registers all MCP tools into the given registry.
func RegisterMCPTools(reg *tool.Registry, manager *mcp.Manager) {
	for _, t := range BuildMCPTools(manager) {
		reg.Register(t)
	}
}
