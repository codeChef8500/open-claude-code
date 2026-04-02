package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"
)

// ── Transport interface ─────────────────────────────────────────────────────

// Transport is the abstraction for communicating with an MCP server.
// All transports must be safe for concurrent use.
type Transport interface {
	// Start initializes the transport connection.
	Start(ctx context.Context) error
	// Send writes a JSON-RPC message to the server.
	Send(msg []byte) error
	// Receive returns a channel that yields incoming JSON-RPC messages.
	Receive() <-chan []byte
	// Close shuts down the transport.
	Close() error
}

// ── Stdio transport ─────────────────────────────────────────────────────────

// StdioTransport communicates with an MCP server via stdin/stdout of a subprocess.
type StdioTransport struct {
	command string
	args    []string
	env     []string

	mu     sync.Mutex
	stdin  io.WriteCloser
	stdout *bufio.Reader
	cmd    *exec.Cmd
	msgCh  chan []byte
	closed chan struct{}
}

// NewStdioTransport creates a transport that spawns a subprocess.
func NewStdioTransport(command string, args, env []string) *StdioTransport {
	return &StdioTransport{
		command: command,
		args:    args,
		env:     env,
		msgCh:   make(chan []byte, 64),
		closed:  make(chan struct{}),
	}
}

// Start spawns the subprocess and starts reading from stdout.
func (t *StdioTransport) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, t.command, t.args...)
	if len(t.env) > 0 {
		cmd.Env = t.env
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdio transport: stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdio transport: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("stdio transport: start: %w", err)
	}

	t.mu.Lock()
	t.cmd = cmd
	t.stdin = stdinPipe
	t.stdout = bufio.NewReader(stdoutPipe)
	t.mu.Unlock()

	go t.readLoop()
	return nil
}

// Send writes a newline-delimited JSON message to the subprocess stdin.
func (t *StdioTransport) Send(msg []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stdin == nil {
		return fmt.Errorf("stdio transport: not started")
	}
	_, err := fmt.Fprintf(t.stdin, "%s\n", msg)
	return err
}

// Receive returns the channel of incoming messages.
func (t *StdioTransport) Receive() <-chan []byte {
	return t.msgCh
}

// Close shuts down the subprocess.
func (t *StdioTransport) Close() error {
	select {
	case <-t.closed:
		return nil
	default:
		close(t.closed)
	}
	t.mu.Lock()
	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	cmd := t.cmd
	t.mu.Unlock()
	if cmd != nil {
		return cmd.Wait()
	}
	return nil
}

func (t *StdioTransport) readLoop() {
	defer close(t.msgCh)
	for {
		select {
		case <-t.closed:
			return
		default:
		}
		line, err := t.stdout.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				slog.Debug("stdio transport: read error", slog.Any("err", err))
			}
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		t.msgCh <- []byte(line)
	}
}

// ── SSE transport (scaffolded) ──────────────────────────────────────────────

// SSETransport connects to an MCP server via Server-Sent Events over HTTP.
// This is a scaffolded implementation; full SSE support requires an HTTP
// client with long-lived connection handling.
type SSETransport struct {
	url     string
	headers map[string]string

	mu       sync.Mutex
	msgCh    chan []byte
	closed   chan struct{}
	client   *http.Client
	postURL  string // discovered from SSE endpoint event
}

// NewSSETransport creates an SSE transport for the given URL.
func NewSSETransport(url string, headers map[string]string) *SSETransport {
	return &SSETransport{
		url:     url,
		headers: headers,
		msgCh:   make(chan []byte, 64),
		closed:  make(chan struct{}),
		client:  &http.Client{},
	}
}

// Start connects to the SSE endpoint and begins reading events.
func (t *SSETransport) Start(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return fmt.Errorf("sse transport: create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("sse transport: connect: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("sse transport: unexpected status %d", resp.StatusCode)
	}

	go t.readSSELoop(resp.Body)
	return nil
}

// Send posts a JSON-RPC message to the server's message endpoint.
func (t *SSETransport) Send(msg []byte) error {
	t.mu.Lock()
	postURL := t.postURL
	t.mu.Unlock()
	if postURL == "" {
		return fmt.Errorf("sse transport: no message endpoint discovered")
	}

	req, err := http.NewRequest(http.MethodPost, postURL, strings.NewReader(string(msg)))
	if err != nil {
		return fmt.Errorf("sse transport: create post request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("sse transport: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sse transport: post status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Receive returns the channel of incoming messages.
func (t *SSETransport) Receive() <-chan []byte {
	return t.msgCh
}

// Close shuts down the SSE connection.
func (t *SSETransport) Close() error {
	select {
	case <-t.closed:
		return nil
	default:
		close(t.closed)
	}
	return nil
}

func (t *SSETransport) readSSELoop(body io.ReadCloser) {
	defer body.Close()
	defer close(t.msgCh)

	scanner := bufio.NewScanner(body)
	var eventType, eventData string

	for scanner.Scan() {
		select {
		case <-t.closed:
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			// Empty line = end of event.
			t.processSSEEvent(eventType, eventData)
			eventType = ""
			eventData = ""
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if eventData != "" {
				eventData += "\n" + data
			} else {
				eventData = data
			}
		}
	}
}

func (t *SSETransport) processSSEEvent(eventType, data string) {
	if data == "" {
		return
	}
	switch eventType {
	case "endpoint":
		// Server tells us the POST endpoint for messages.
		t.mu.Lock()
		t.postURL = data
		t.mu.Unlock()
		slog.Debug("sse transport: discovered endpoint", slog.String("url", data))

	case "message":
		// JSON-RPC message from server.
		t.msgCh <- []byte(data)

	default:
		// Unknown event type — treat as a message if it looks like JSON.
		if json.Valid([]byte(data)) {
			t.msgCh <- []byte(data)
		}
	}
}

// ── HTTP Streamable transport (scaffolded) ──────────────────────────────────

// HTTPTransport connects to an MCP server via the Streamable HTTP transport.
// Per the MCP spec, this uses POST for sending and can receive responses via
// SSE or direct JSON.
type HTTPTransport struct {
	url     string
	headers map[string]string

	mu        sync.Mutex
	sessionID string
	msgCh     chan []byte
	closed    chan struct{}
	client    *http.Client
}

// NewHTTPTransport creates an HTTP Streamable transport.
func NewHTTPTransport(url string, headers map[string]string) *HTTPTransport {
	return &HTTPTransport{
		url:     url,
		headers: headers,
		msgCh:   make(chan []byte, 64),
		closed:  make(chan struct{}),
		client:  &http.Client{},
	}
}

// Start initializes the HTTP transport (no persistent connection needed).
func (t *HTTPTransport) Start(_ context.Context) error {
	return nil // HTTP is stateless; connection happens on Send.
}

// Send posts a JSON-RPC message and reads the response.
func (t *HTTPTransport) Send(msg []byte) error {
	req, err := http.NewRequest(http.MethodPost, t.url, strings.NewReader(string(msg)))
	if err != nil {
		return fmt.Errorf("http transport: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	t.mu.Lock()
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}
	t.mu.Unlock()

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("http transport: post: %w", err)
	}
	defer resp.Body.Close()

	// Capture session ID from response headers.
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.sessionID = sid
		t.mu.Unlock()
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http transport: status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body as JSON-RPC messages.
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		// SSE response — read events.
		go func() {
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					data := strings.TrimPrefix(line, "data: ")
					if json.Valid([]byte(data)) {
						t.msgCh <- []byte(data)
					}
				}
			}
		}()
	} else {
		// Direct JSON response.
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("http transport: read body: %w", err)
		}
		if len(body) > 0 && json.Valid(body) {
			t.msgCh <- body
		}
	}
	return nil
}

// Receive returns the channel of incoming messages.
func (t *HTTPTransport) Receive() <-chan []byte {
	return t.msgCh
}

// Close shuts down the HTTP transport.
func (t *HTTPTransport) Close() error {
	select {
	case <-t.closed:
		return nil
	default:
		close(t.closed)
	}
	return nil
}

// ── WebSocket transport (scaffolded) ────────────────────────────────────────

// WebSocketTransportConfig holds configuration for a WebSocket transport.
// This is scaffolded; actual WebSocket implementation requires a ws library.
type WebSocketTransportConfig struct {
	URL     string
	Headers map[string]string
}

// NewWebSocketTransportConfig creates a WebSocket transport config.
func NewWebSocketTransportConfig(url string, headers map[string]string) *WebSocketTransportConfig {
	return &WebSocketTransportConfig{URL: url, Headers: headers}
}

// ── Transport factory ───────────────────────────────────────────────────────

// NewTransportFromConfig creates the appropriate transport for a server config.
func NewTransportFromConfig(cfg *ServerConfig) (Transport, error) {
	expanded := cfg.ExpandEnv()
	switch expanded.Transport {
	case TransportStdio, "":
		return NewStdioTransport(expanded.Command, expanded.Args, expanded.Env), nil
	case TransportSSE, TransportSSEIDE:
		return NewSSETransport(expanded.URL, expanded.Headers), nil
	case TransportHTTP:
		return NewHTTPTransport(expanded.URL, expanded.Headers), nil
	default:
		return nil, fmt.Errorf("unsupported transport: %s", expanded.Transport)
	}
}
