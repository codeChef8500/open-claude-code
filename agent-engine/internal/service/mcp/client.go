package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

// Client is a connection to one MCP server via stdio or SSE transport.
// Currently only stdio is implemented; SSE support is scaffolded.
type Client struct {
	cfg          ServerConfig
	info         ServerInfo
	caps         Caps
	tools        []MCPTool
	resources    []MCPResource
	instructions string
	state        ConnectionState
	stateError   string

	mu      sync.Mutex
	nextID  atomic.Int64
	pending map[int64]chan *Response

	stdin  io.WriteCloser
	stdout *bufio.Reader
	cmd    *exec.Cmd
	closed chan struct{}

	// notificationHandler is called for server-initiated notifications.
	notificationHandler func(method string, params json.RawMessage)
	// samplingHandler is called for sampling/createMessage requests.
	samplingHandler func(ctx context.Context, req *Request) (*Response, error)
}

// NewClient creates a Client from a ServerConfig but does not connect yet.
func NewClient(cfg ServerConfig) *Client {
	return &Client{
		cfg:     cfg,
		state:   StatePending,
		pending: make(map[int64]chan *Response),
		closed:  make(chan struct{}),
	}
}

// Connect starts the server subprocess (stdio) and performs the MCP handshake.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	expanded := c.cfg.ExpandEnv()
	if expanded.Transport == TransportSSE {
		return fmt.Errorf("mcp: SSE transport not yet implemented")
	}

	cmd := exec.CommandContext(ctx, expanded.Command, expanded.Args...)
	cmd.Env = expanded.Env

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp %s: stdin pipe: %w", c.cfg.Name, err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp %s: stdout pipe: %w", c.cfg.Name, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mcp %s: start: %w", c.cfg.Name, err)
	}

	c.cmd = cmd
	c.stdin = stdinPipe
	c.stdout = bufio.NewReader(stdoutPipe)

	// Start reader goroutine.
	go c.readLoop()

	// Perform the initialize handshake.
	initParams := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		ClientInfo:      ClientInfo{Name: "agent-engine", Version: "1.0.0"},
		Capabilities:    Caps{Tools: &ToolsCap{}},
	}
	var initResult InitializeResult
	if err := c.call(ctx, MethodInitialize, initParams, &initResult); err != nil {
		return fmt.Errorf("mcp %s: initialize: %w", c.cfg.Name, err)
	}
	c.info = initResult.ServerInfo
	c.caps = initResult.Capabilities

	// Capture server instructions if provided.
	// Note: instructions may come as part of the initialize result in future MCP versions.

	// Notify server we are ready.
	if err := c.notify(MethodInitialized, nil); err != nil {
		slog.Warn("mcp: initialized notification failed", slog.String("server", c.cfg.Name), slog.Any("err", err))
	}

	// Pre-fetch tool list.
	if err := c.refreshTools(ctx); err != nil {
		slog.Warn("mcp: initial tool list fetch failed", slog.String("server", c.cfg.Name), slog.Any("err", err))
	}

	// Pre-fetch resource list if supported.
	if c.caps.Resources != nil {
		if err := c.refreshResources(ctx); err != nil {
			slog.Warn("mcp: initial resource list fetch failed", slog.String("server", c.cfg.Name), slog.Any("err", err))
		}
	}

	c.state = StateConnected
	c.stateError = ""

	slog.Info("mcp: connected",
		slog.String("server", c.cfg.Name),
		slog.String("server_version", c.info.Version),
		slog.Int("tools", len(c.tools)),
		slog.Int("resources", len(c.resources)))
	return nil
}

// Close shuts down the server subprocess.
func (c *Client) Close() error {
	select {
	case <-c.closed:
		return nil
	default:
		close(c.closed)
	}
	c.mu.Lock()
	c.state = StateDisabled
	c.mu.Unlock()
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil {
		return c.cmd.Wait()
	}
	return nil
}

// Reconnect closes and reopens the connection.
func (c *Client) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	c.state = StatePending
	c.mu.Unlock()

	// Close existing connection.
	_ = c.Close()

	// Reset closed channel and pending map.
	c.mu.Lock()
	c.closed = make(chan struct{})
	c.pending = make(map[int64]chan *Response)
	c.mu.Unlock()

	return c.Connect(ctx)
}

// Name returns the logical server name.
func (c *Client) Name() string { return c.cfg.Name }

// ServerInfo returns the server's self-reported identity.
func (c *Client) ServerInfo() ServerInfo { return c.info }

// Config returns the server config.
func (c *Client) Config() ServerConfig { return c.cfg }

// ConnectionState returns the current connection state.
func (c *Client) ConnectionState() ConnectionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// SetConnectionState sets the connection state.
func (c *Client) SetConnectionState(state ConnectionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = state
}

// ConnectionInfo returns the full connection status.
func (c *Client) ConnectionInfo() MCPServerConnection {
	c.mu.Lock()
	defer c.mu.Unlock()
	conn := MCPServerConnection{
		Name:   c.cfg.Name,
		State:  c.state,
		Config: c.cfg,
		Error:  c.stateError,
	}
	if c.state == StateConnected {
		conn.ServerInfo = &c.info
		conn.Capabilities = &c.caps
		conn.Instructions = c.instructions
	}
	return conn
}

// Instructions returns the server-provided instructions string.
func (c *Client) Instructions() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.instructions
}

// Capabilities returns the negotiated capabilities.
func (c *Client) Capabilities() Caps {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.caps
}

// Tools returns the cached tool list.
func (c *Client) Tools() []MCPTool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tools
}

// ToolsCapped returns the cached tool list with descriptions capped
// to MaxMCPDescriptionLength.
func (c *Client) ToolsCapped() []MCPTool {
	c.mu.Lock()
	tools := make([]MCPTool, len(c.tools))
	copy(tools, c.tools)
	c.mu.Unlock()

	for i := range tools {
		if len(tools[i].Description) > MaxMCPDescriptionLength {
			tools[i].Description = tools[i].Description[:MaxMCPDescriptionLength] + "..."
		}
	}
	return tools
}

// Resources returns the cached resource list.
func (c *Client) Resources() []MCPResource {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resources
}

// SetNotificationHandler registers a handler for server notifications.
func (c *Client) SetNotificationHandler(h func(method string, params json.RawMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notificationHandler = h
}

// SetSamplingHandler registers a handler for sampling/createMessage requests.
func (c *Client) SetSamplingHandler(h func(ctx context.Context, req *Request) (*Response, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.samplingHandler = h
}

// CallTool invokes a named tool on the MCP server.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (*CallToolResult, error) {
	params := CallToolParams{Name: name, Arguments: args}
	var result CallToolResult
	if err := c.call(ctx, MethodCallTool, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListResources returns the server's resource list.
func (c *Client) ListResources(ctx context.Context) ([]MCPResource, error) {
	var result ListResourcesResult
	if err := c.call(ctx, MethodListResources, nil, &result); err != nil {
		return nil, err
	}
	return result.Resources, nil
}

// ReadResource reads a resource by URI.
func (c *Client) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	params := ReadResourceParams{URI: uri}
	var result ReadResourceResult
	if err := c.call(ctx, MethodReadResource, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Internal helpers ───────────────────────────────────────────────────────

func (c *Client) refreshTools(ctx context.Context) error {
	var result ListToolsResult
	if err := c.call(ctx, MethodListTools, nil, &result); err != nil {
		return err
	}
	c.mu.Lock()
	c.tools = result.Tools
	c.mu.Unlock()
	return nil
}

func (c *Client) refreshResources(ctx context.Context) error {
	var result ListResourcesResult
	if err := c.call(ctx, MethodListResources, nil, &result); err != nil {
		return err
	}
	c.mu.Lock()
	c.resources = result.Resources
	c.mu.Unlock()
	return nil
}

// RefreshTools re-fetches the tool list from the server.
func (c *Client) RefreshTools(ctx context.Context) error {
	return c.refreshTools(ctx)
}

// RefreshResources re-fetches the resource list from the server.
func (c *Client) RefreshResources(ctx context.Context) error {
	return c.refreshResources(ctx)
}

// ListPrompts returns the server's prompt list.
func (c *Client) ListPrompts(ctx context.Context) ([]MCPPrompt, error) {
	var result ListPromptsResult
	if err := c.call(ctx, MethodListPrompts, nil, &result); err != nil {
		return nil, err
	}
	return result.Prompts, nil
}

// GetPrompt retrieves a prompt by name.
func (c *Client) GetPrompt(ctx context.Context, name string, args json.RawMessage) (*GetPromptResult, error) {
	params := GetPromptParams{Name: name, Arguments: args}
	var result GetPromptResult
	if err := c.call(ctx, MethodGetPrompt, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) call(ctx context.Context, method string, params interface{}, out interface{}) error {
	id := c.nextID.Add(1)
	rawParams, err := marshalParams(params)
	if err != nil {
		return err
	}
	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	replyCh := make(chan *Response, 1)
	c.mu.Lock()
	c.pending[id] = replyCh
	c.mu.Unlock()

	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("write request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return ctx.Err()
	case resp := <-replyCh:
		if resp.Error != nil {
			return resp.Error
		}
		if out != nil && resp.Result != nil {
			return json.Unmarshal(resp.Result, out)
		}
		return nil
	}
}

func (c *Client) notify(method string, params interface{}) error {
	rawParams, err := marshalParams(params)
	if err != nil {
		return err
	}
	req := Request{JSONRPC: "2.0", Method: method, Params: rawParams}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}

func (c *Client) readLoop() {
	for {
		select {
		case <-c.closed:
			return
		default:
		}
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				slog.Debug("mcp read error", slog.String("server", c.cfg.Name), slog.Any("err", err))
			}
			return
		}
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			slog.Debug("mcp: invalid JSON from server", slog.String("server", c.cfg.Name))
			continue
		}
		if resp.ID == nil {
			// Server notification — route to handler.
			c.handleServerNotification(line)
			continue
		}
		id, ok := parseID(resp.ID)
		if !ok {
			continue
		}
		c.mu.Lock()
		ch, found := c.pending[id]
		if found {
			delete(c.pending, id)
		}
		c.mu.Unlock()
		if found {
			ch <- &resp
		}
	}
}

// handleServerNotification routes a server-initiated notification or request.
func (c *Client) handleServerNotification(rawLine string) {
	// Parse as a request (notifications use the Request shape without an ID).
	var req Request
	if err := json.Unmarshal([]byte(rawLine), &req); err != nil {
		return
	}

	switch req.Method {
	case MethodToolsListChanged:
		// Server's tool list changed — refresh.
		slog.Debug("mcp: tools list changed notification", slog.String("server", c.cfg.Name))
		go func() {
			ctx := context.Background()
			if err := c.refreshTools(ctx); err != nil {
				slog.Warn("mcp: refresh tools after notification failed",
					slog.String("server", c.cfg.Name), slog.Any("err", err))
			}
		}()

	case MethodResourcesListChanged:
		// Server's resource list changed — refresh.
		slog.Debug("mcp: resources list changed notification", slog.String("server", c.cfg.Name))
		go func() {
			ctx := context.Background()
			if err := c.refreshResources(ctx); err != nil {
				slog.Warn("mcp: refresh resources after notification failed",
					slog.String("server", c.cfg.Name), slog.Any("err", err))
			}
		}()

	case MethodSamplingCreateMessage:
		// Server wants us to create a message (sampling request).
		c.mu.Lock()
		handler := c.samplingHandler
		c.mu.Unlock()
		if handler != nil && req.ID != nil {
			go func() {
				ctx := context.Background()
				resp, err := handler(ctx, &req)
				if err != nil {
					slog.Warn("mcp: sampling handler error",
						slog.String("server", c.cfg.Name), slog.Any("err", err))
					return
				}
				if resp != nil {
					data, _ := json.Marshal(resp)
					c.mu.Lock()
					_, _ = fmt.Fprintf(c.stdin, "%s\n", data)
					c.mu.Unlock()
				}
			}()
		}

	default:
		// Route to custom notification handler if registered.
		c.mu.Lock()
		handler := c.notificationHandler
		c.mu.Unlock()
		if handler != nil {
			handler(req.Method, req.Params)
		}
	}
}

// IsMcpSessionExpiredError detects whether an error is an MCP "Session not found"
// error (HTTP 404 + JSON-RPC code -32001).
func IsMcpSessionExpiredError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, "404") {
		return false
	}
	return strings.Contains(msg, `"code":-32001`) || strings.Contains(msg, `"code": -32001`)
}

func parseID(raw interface{}) (int64, bool) {
	switch v := raw.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	}
	return 0, false
}

func marshalParams(params interface{}) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	return json.Marshal(params)
}
