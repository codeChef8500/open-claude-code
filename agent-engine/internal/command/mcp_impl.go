package command

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// /mcp — full implementation
// Aligned with claude-code-main commands/mcp/mcp.tsx.
//
// Subcommands: list, add, remove, restart
// Interactive mode returns structured data for TUI MCP management panel.
// ──────────────────────────────────────────────────────────────────────────────

// MCPPanelData is the structured data for the MCP management TUI component.
type MCPPanelData struct {
	Subcommand string          `json:"subcommand"` // "list", "add", "remove", "restart"
	Servers    []MCPServerView `json:"servers,omitempty"`
	// For add subcommand
	ServerName   string                 `json:"server_name,omitempty"`
	ServerConfig map[string]interface{} `json:"server_config,omitempty"`
	// Result message
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// MCPServerView is a display-friendly view of an MCP server.
type MCPServerView struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`    // "connected", "disconnected", "error"
	Transport string   `json:"transport"` // "stdio", "sse", "streamable-http"
	ToolCount int      `json:"tool_count"`
	Tools     []string `json:"tools,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// DeepMcpCommand replaces the basic McpCommand with full logic.
type DeepMcpCommand struct{ BaseCommand }

func (c *DeepMcpCommand) Name() string                  { return "mcp" }
func (c *DeepMcpCommand) Description() string           { return "Manage MCP servers" }
func (c *DeepMcpCommand) ArgumentHint() string          { return "[list|add|remove|restart]" }
func (c *DeepMcpCommand) IsImmediate() bool             { return true }
func (c *DeepMcpCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepMcpCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepMcpCommand) ExecuteInteractive(ctx context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &MCPPanelData{Subcommand: "list"}

	if len(args) > 0 {
		data.Subcommand = strings.ToLower(args[0])
	}

	svc := getMCPService(ectx)

	switch data.Subcommand {
	case "list", "":
		data.Subcommand = "list"
		data.Servers = listMCPServers(svc, ectx)

	case "add":
		if len(args) < 2 {
			data.Error = "Usage: /mcp add <server-name> [config-json]"
		} else {
			data.ServerName = args[1]
			config := map[string]interface{}{}
			if len(args) > 2 {
				// Simple key=value parsing for inline config
				for _, arg := range args[2:] {
					parts := strings.SplitN(arg, "=", 2)
					if len(parts) == 2 {
						config[parts[0]] = parts[1]
					}
				}
			}
			data.ServerConfig = config
			if svc != nil {
				if err := svc.AddServer(data.ServerName, config); err != nil {
					data.Error = fmt.Sprintf("Failed to add server: %v", err)
				} else {
					data.Message = fmt.Sprintf("Server '%s' added successfully", data.ServerName)
					data.Servers = listMCPServers(svc, ectx)
				}
			} else {
				data.Error = "MCP service not available"
			}
		}

	case "remove", "rm", "delete":
		data.Subcommand = "remove"
		if len(args) < 2 {
			data.Error = "Usage: /mcp remove <server-name>"
		} else {
			name := args[1]
			if svc != nil {
				if err := svc.RemoveServer(name); err != nil {
					data.Error = fmt.Sprintf("Failed to remove server: %v", err)
				} else {
					data.Message = fmt.Sprintf("Server '%s' removed", name)
					data.Servers = listMCPServers(svc, ectx)
				}
			} else {
				data.Error = "MCP service not available"
			}
		}

	case "restart":
		if len(args) < 2 {
			data.Error = "Usage: /mcp restart <server-name>"
		} else {
			name := args[1]
			if svc != nil {
				if err := svc.RestartServer(ctx, name); err != nil {
					data.Error = fmt.Sprintf("Failed to restart server: %v", err)
				} else {
					data.Message = fmt.Sprintf("Server '%s' restarted", name)
					data.Servers = listMCPServers(svc, ectx)
				}
			} else {
				data.Error = "MCP service not available"
			}
		}

	default:
		data.Subcommand = "list"
		data.Servers = listMCPServers(svc, ectx)
	}

	return &InteractiveResult{
		Component: "mcp",
		Data:      data,
	}, nil
}

// getMCPService extracts MCPService from ExecContext safely.
func getMCPService(ectx *ExecContext) MCPService {
	if ectx == nil || ectx.Services == nil {
		return nil
	}
	return ectx.Services.MCP
}

// listMCPServers builds the server list view from available services.
func listMCPServers(svc MCPService, ectx *ExecContext) []MCPServerView {
	if svc == nil {
		// Fallback: use ActiveMCPServers from ExecContext
		if ectx != nil && len(ectx.ActiveMCPServers) > 0 {
			views := make([]MCPServerView, len(ectx.ActiveMCPServers))
			for i, name := range ectx.ActiveMCPServers {
				views[i] = MCPServerView{
					Name:   name,
					Status: "connected",
				}
			}
			return views
		}
		return nil
	}

	servers := svc.ListServers()
	views := make([]MCPServerView, len(servers))
	for i, s := range servers {
		view := MCPServerView{
			Name:      s.Name,
			Status:    s.Status,
			Transport: s.Transport,
			ToolCount: s.ToolCount,
			Error:     s.Error,
		}
		// Fetch tool names if available
		tools := svc.GetServerTools(s.Name)
		for _, t := range tools {
			if name, ok := t.(string); ok {
				view.Tools = append(view.Tools, name)
			}
		}
		views[i] = view
	}
	return views
}

func init() {
	defaultRegistry.RegisterOrReplace(&DeepMcpCommand{})
}
