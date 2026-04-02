package mcpauth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/service/mcp"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// AuthProvider is the interface for MCP OAuth authentication.
// The actual implementation handles browser-based OAuth flows.
type AuthProvider interface {
	// StartAuthFlow initiates the OAuth flow for an MCP server.
	// Returns an authorization URL for the user to visit.
	StartAuthFlow(ctx context.Context, serverName string) (authURL string, err error)
	// CompleteAuthFlow completes the OAuth flow after user authorization.
	CompleteAuthFlow(ctx context.Context, serverName string, code string) error
	// IsAuthenticated checks if a server already has valid credentials.
	IsAuthenticated(serverName string) bool
	// ClearAuth removes stored credentials for a server.
	ClearAuth(serverName string) error
}

// McpAuthTool handles OAuth authentication for MCP servers that require it.
type McpAuthTool struct {
	tool.BaseTool
	manager  *mcp.Manager
	provider AuthProvider
}

// New creates an McpAuthTool.
func New(manager *mcp.Manager, provider AuthProvider) *McpAuthTool {
	return &McpAuthTool{
		manager:  manager,
		provider: provider,
	}
}

func (t *McpAuthTool) Name() string           { return "mcp_auth" }
func (t *McpAuthTool) UserFacingName() string  { return "McpAuth" }
func (t *McpAuthTool) Description() string {
	return "Authenticate with an MCP server that requires OAuth. Lists servers needing auth or initiates the auth flow."
}
func (t *McpAuthTool) MaxResultSizeChars() int                  { return 10_000 }
func (t *McpAuthTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *McpAuthTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *McpAuthTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *McpAuthTool) SearchHint() string                       { return "authenticate with MCP servers requiring OAuth" }

func (t *McpAuthTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["list", "login", "logout", "status"],
				"description": "Action to perform: list servers needing auth, login to a server, logout from a server, or check auth status."
			},
			"server": {
				"type": "string",
				"description": "Server name (required for login/logout/status)."
			}
		},
		"required": ["action"]
	}`)
}

func (t *McpAuthTool) Prompt(_ *tool.UseContext) string {
	return `Use this tool to manage authentication for MCP servers that require OAuth.
Actions:
  - list: Show all servers that need authentication.
  - login <server>: Start the OAuth flow for a server.
  - logout <server>: Clear credentials for a server.
  - status <server>: Check if a server is authenticated.`
}

func (t *McpAuthTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *McpAuthTool) Call(ctx context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)

	go func() {
		defer close(ch)

		var args struct {
			Action string `json:"action"`
			Server string `json:"server"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "invalid input: " + err.Error(), IsError: true}
			return
		}

		switch args.Action {
		case "list":
			t.handleList(ch)
		case "login":
			t.handleLogin(ctx, args.Server, ch)
		case "logout":
			t.handleLogout(args.Server, ch)
		case "status":
			t.handleStatus(args.Server, ch)
		default:
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Unknown action %q. Use: list, login, logout, status.", args.Action),
				IsError: true,
			}
		}
	}()

	return ch, nil
}

func (t *McpAuthTool) handleList(ch chan<- *engine.ContentBlock) {
	states := t.manager.ConnectionStates()
	var needsAuth []string
	var authenticated []string
	var other []string

	for _, s := range states {
		switch s.State {
		case mcp.StateNeedsAuth:
			needsAuth = append(needsAuth, s.Name)
		case mcp.StateConnected:
			authenticated = append(authenticated, s.Name)
		default:
			other = append(other, fmt.Sprintf("%s (%s)", s.Name, s.State))
		}
	}

	var parts []string
	if len(needsAuth) > 0 {
		parts = append(parts, "Servers needing authentication:\n  - "+strings.Join(needsAuth, "\n  - "))
	}
	if len(authenticated) > 0 {
		parts = append(parts, "Authenticated servers:\n  - "+strings.Join(authenticated, "\n  - "))
	}
	if len(other) > 0 {
		parts = append(parts, "Other servers:\n  - "+strings.Join(other, "\n  - "))
	}
	if len(parts) == 0 {
		parts = append(parts, "No MCP servers configured.")
	}

	ch <- &engine.ContentBlock{
		Type: engine.ContentTypeText,
		Text: strings.Join(parts, "\n\n"),
	}
}

func (t *McpAuthTool) handleLogin(ctx context.Context, server string, ch chan<- *engine.ContentBlock) {
	if server == "" {
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "server name is required for login", IsError: true}
		return
	}

	if t.provider == nil {
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    "OAuth authentication is not available. No auth provider configured.",
			IsError: true,
		}
		return
	}

	if t.provider.IsAuthenticated(server) {
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Server %q is already authenticated. Use logout first to re-authenticate.", server),
		}
		return
	}

	authURL, err := t.provider.StartAuthFlow(ctx, server)
	if err != nil {
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    fmt.Sprintf("Failed to start auth flow for %q: %v", server, err),
			IsError: true,
		}
		return
	}

	ch <- &engine.ContentBlock{
		Type: engine.ContentTypeText,
		Text: fmt.Sprintf("Please visit the following URL to authenticate with %q:\n\n%s\n\nAfter authorization, the server will reconnect automatically.", server, authURL),
	}
}

func (t *McpAuthTool) handleLogout(server string, ch chan<- *engine.ContentBlock) {
	if server == "" {
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "server name is required for logout", IsError: true}
		return
	}

	if t.provider == nil {
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    "OAuth authentication is not available. No auth provider configured.",
			IsError: true,
		}
		return
	}

	if err := t.provider.ClearAuth(server); err != nil {
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    fmt.Sprintf("Failed to clear auth for %q: %v", server, err),
			IsError: true,
		}
		return
	}

	ch <- &engine.ContentBlock{
		Type: engine.ContentTypeText,
		Text: fmt.Sprintf("Cleared authentication for %q. Use login to re-authenticate.", server),
	}
}

func (t *McpAuthTool) handleStatus(server string, ch chan<- *engine.ContentBlock) {
	if server == "" {
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "server name is required for status", IsError: true}
		return
	}

	info, found := t.manager.GetConnectionInfo(server)
	if !found {
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    fmt.Sprintf("Server %q not found.", server),
			IsError: true,
		}
		return
	}

	var status string
	switch info.State {
	case mcp.StateConnected:
		status = "authenticated and connected"
	case mcp.StateNeedsAuth:
		status = "needs authentication (use login action)"
	case mcp.StateFailed:
		status = fmt.Sprintf("failed: %s", info.Error)
	case mcp.StatePending:
		status = "connecting..."
	case mcp.StateDisabled:
		status = "disabled"
	default:
		status = string(info.State)
	}

	ch <- &engine.ContentBlock{
		Type: engine.ContentTypeText,
		Text: fmt.Sprintf("Server %q: %s", server, status),
	}
}
