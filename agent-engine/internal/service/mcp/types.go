package mcp

import "encoding/json"

// ── JSON-RPC 2.0 primitives ────────────────────────────────────────────────

type JSONRPCID interface{} // string | int | nil

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      JSONRPCID       `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      JSONRPCID       `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return e.Message }

// ── MCP Protocol types ─────────────────────────────────────────────────────

// ServerInfo is returned during the initialize handshake.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientInfo identifies this client during the initialize handshake.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeParams are sent by the client during the handshake.
type InitializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ClientInfo      ClientInfo `json:"clientInfo"`
	Capabilities    Caps       `json:"capabilities"`
}

// InitializeResult is the server's response to initialize.
type InitializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ServerInfo      ServerInfo `json:"serverInfo"`
	Capabilities    Caps       `json:"capabilities"`
}

// Caps describes negotiated protocol capabilities.
type Caps struct {
	Tools     *ToolsCap     `json:"tools,omitempty"`
	Resources *ResourcesCap `json:"resources,omitempty"`
	Prompts   *PromptsCap   `json:"prompts,omitempty"`
}

type ToolsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCap struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type PromptsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ── Tools ──────────────────────────────────────────────────────────────────

// MCPTool describes a tool exposed by an MCP server.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ListToolsResult is the response to tools/list.
type ListToolsResult struct {
	Tools      []MCPTool `json:"tools"`
	NextCursor string    `json:"nextCursor,omitempty"`
}

// CallToolParams are the params for tools/call.
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ContentItem is a single content item in a tool call result.
type ContentItem struct {
	Type     string `json:"type"` // "text"|"image"|"resource"
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64 for image
	URI      string `json:"uri,omitempty"`
}

// CallToolResult is the response to tools/call.
type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ── Resources ─────────────────────────────────────────────────────────────

// MCPResource describes a resource exposed by an MCP server.
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ListResourcesResult is the response to resources/list.
type ListResourcesResult struct {
	Resources  []MCPResource `json:"resources"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

// ReadResourceParams are the params for resources/read.
type ReadResourceParams struct {
	URI string `json:"uri"`
}

// ReadResourceResult is the response to resources/read.
type ReadResourceResult struct {
	Contents []ContentItem `json:"contents"`
}

// ── Transport constants ────────────────────────────────────────────────────

const (
	TransportStdio     = "stdio"
	TransportSSE       = "sse"
	TransportSSEIDE    = "sse-ide"
	TransportHTTP      = "http"
	TransportWebSocket = "ws"
	TransportSDK       = "sdk"

	ProtocolVersion = "2024-11-05"

	MethodInitialize            = "initialize"
	MethodInitialized           = "notifications/initialized"
	MethodListTools             = "tools/list"
	MethodCallTool              = "tools/call"
	MethodListResources         = "resources/list"
	MethodReadResource          = "resources/read"
	MethodListPrompts           = "prompts/list"
	MethodGetPrompt             = "prompts/get"
	MethodSamplingCreateMessage = "sampling/createMessage"
	MethodToolsListChanged      = "notifications/tools/list_changed"
	MethodResourcesListChanged  = "notifications/resources/list_changed"
	MethodRootsList             = "roots/list"
)

// ── Limits & defaults ──────────────────────────────────────────────────────

const (
	// MaxMCPDescriptionLength caps tool descriptions sent to the model.
	MaxMCPDescriptionLength = 2048

	// DefaultMCPToolTimeoutSec is the default timeout for MCP tool calls (~27.8 hours).
	DefaultMCPToolTimeoutSec = 100_000

	// DefaultMCPConnectionTimeoutSec is the default connection timeout.
	DefaultMCPConnectionTimeoutSec = 30

	// DefaultMCPRequestTimeoutSec is the default per-request timeout.
	DefaultMCPRequestTimeoutSec = 60

	// MaxMCPToolResultChars caps tool result text before truncation.
	MaxMCPToolResultChars = 100_000
)

// ── Connection state ───────────────────────────────────────────────────────

// ConnectionState represents the lifecycle state of an MCP server connection.
type ConnectionState string

const (
	StateConnected ConnectionState = "connected"
	StateFailed    ConnectionState = "failed"
	StateNeedsAuth ConnectionState = "needs-auth"
	StatePending   ConnectionState = "pending"
	StateDisabled  ConnectionState = "disabled"
)

// MCPServerConnection describes the full state of a server connection.
type MCPServerConnection struct {
	Name         string          `json:"name"`
	State        ConnectionState `json:"type"`
	Config       ServerConfig    `json:"config"`
	ServerInfo   *ServerInfo     `json:"serverInfo,omitempty"`
	Capabilities *Caps           `json:"capabilities,omitempty"`
	Error        string          `json:"error,omitempty"`
	Instructions string          `json:"instructions,omitempty"`
	// ReconnectAttempt tracks the current reconnect iteration (Pending state).
	ReconnectAttempt     int `json:"reconnectAttempt,omitempty"`
	MaxReconnectAttempts int `json:"maxReconnectAttempts,omitempty"`
}

// ── Config scoping ─────────────────────────────────────────────────────────

// ConfigScope identifies where an MCP server config was loaded from.
type ConfigScope string

const (
	ScopeLocal      ConfigScope = "local"
	ScopeUser       ConfigScope = "user"
	ScopeProject    ConfigScope = "project"
	ScopeDynamic    ConfigScope = "dynamic"
	ScopeEnterprise ConfigScope = "enterprise"
)

// ScopedServerConfig is a ServerConfig annotated with its origin scope.
type ScopedServerConfig struct {
	ServerConfig
	Scope ConfigScope `json:"scope"`
}

// ── Prompts ────────────────────────────────────────────────────────────────

// MCPPrompt describes a prompt template exposed by an MCP server.
type MCPPrompt struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Arguments   json.RawMessage `json:"arguments,omitempty"`
}

// ListPromptsResult is the response to prompts/list.
type ListPromptsResult struct {
	Prompts    []MCPPrompt `json:"prompts"`
	NextCursor string      `json:"nextCursor,omitempty"`
}

// GetPromptParams are the params for prompts/get.
type GetPromptParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// PromptMessage is a single message in a prompt get result.
type PromptMessage struct {
	Role    string      `json:"role"`
	Content ContentItem `json:"content"`
}

// GetPromptResult is the response to prompts/get.
type GetPromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// ── CLI state serialization ────────────────────────────────────────────────

// SerializedTool is a tool descriptor for CLI state exchange.
type SerializedTool struct {
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	InputJSONSchema  json.RawMessage `json:"inputJSONSchema,omitempty"`
	IsMcp            bool            `json:"isMcp,omitempty"`
	OriginalToolName string          `json:"originalToolName,omitempty"`
}

// SerializedClient is a client descriptor for CLI state exchange.
type SerializedClient struct {
	Name         string          `json:"name"`
	Type         ConnectionState `json:"type"`
	Capabilities *Caps           `json:"capabilities,omitempty"`
}

// ServerResource is a resource annotated with its originating server.
type ServerResource struct {
	MCPResource
	Server string `json:"server"`
}

// MCPCliState is the full MCP state snapshot for CLI exchange.
type MCPCliState struct {
	Clients         []SerializedClient          `json:"clients"`
	Configs         map[string]ServerConfig     `json:"configs"`
	Tools           []SerializedTool            `json:"tools"`
	Resources       map[string][]ServerResource `json:"resources"`
	NormalizedNames map[string]string           `json:"normalizedNames,omitempty"`
}
