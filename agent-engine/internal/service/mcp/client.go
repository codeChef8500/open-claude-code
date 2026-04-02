package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client is a connection to one MCP server via stdio or SSE transport.
// Currently only stdio is implemented; SSE support is scaffolded.
type Client struct {
	cfg     ServerConfig
	info    ServerInfo
	caps    Caps
	tools   []MCPTool

	mu      sync.Mutex
	nextID  atomic.Int64
	pending map[int64]chan *Response

	stdin  io.WriteCloser
	stdout *bufio.Reader
	cmd    *exec.Cmd
	closed chan struct{}
}

// NewClient creates a Client from a ServerConfig but does not connect yet.
func NewClient(cfg ServerConfig) *Client {
	return &Client{
		cfg:     cfg,
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

	// Notify server we are ready.
	if err := c.notify(MethodInitialized, nil); err != nil {
		slog.Warn("mcp: initialized notification failed", slog.String("server", c.cfg.Name), slog.Any("err", err))
	}

	// Pre-fetch tool list.
	if err := c.refreshTools(ctx); err != nil {
		slog.Warn("mcp: initial tool list fetch failed", slog.String("server", c.cfg.Name), slog.Any("err", err))
	}

	slog.Info("mcp: connected",
		slog.String("server", c.cfg.Name),
		slog.String("server_version", c.info.Version),
		slog.Int("tools", len(c.tools)))
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
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil {
		return c.cmd.Wait()
	}
	return nil
}

// Name returns the logical server name.
func (c *Client) Name() string { return c.cfg.Name }

// ServerInfo returns the server's self-reported identity.
func (c *Client) ServerInfo() ServerInfo { return c.info }

// Tools returns the cached tool list.
func (c *Client) Tools() []MCPTool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tools
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
			// Notification — ignore for now.
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
