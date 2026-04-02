package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// Manager maintains a pool of MCP client connections and provides a unified
// interface for tool listing and invocation across all connected servers.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

// NewManager creates an empty Manager.
func NewManager() *Manager {
	return &Manager{clients: make(map[string]*Client)}
}

// Connect starts a new server connection from config and registers it.
// Returns an error if a server with the same name is already registered.
func (m *Manager) Connect(ctx context.Context, cfg ServerConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	if _, exists := m.clients[cfg.Name]; exists {
		m.mu.Unlock()
		return fmt.Errorf("mcp manager: server %q already connected", cfg.Name)
	}
	c := NewClient(cfg)
	m.clients[cfg.Name] = c
	m.mu.Unlock()

	if err := c.Connect(ctx); err != nil {
		m.mu.Lock()
		delete(m.clients, cfg.Name)
		m.mu.Unlock()
		return fmt.Errorf("mcp manager: connect %q: %w", cfg.Name, err)
	}
	return nil
}

// Disconnect closes and removes a named server connection.
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	c, ok := m.clients[name]
	if ok {
		delete(m.clients, name)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("mcp manager: server %q not found", name)
	}
	return c.Close()
}

// ConnectAll starts all non-disabled servers in the global config.
// Failures are logged but do not abort remaining servers.
func (m *Manager) ConnectAll(ctx context.Context, cfg GlobalMCPConfig) {
	for _, srv := range cfg.Active() {
		if err := m.Connect(ctx, srv); err != nil {
			slog.Warn("mcp: failed to connect server",
				slog.String("name", srv.Name),
				slog.Any("err", err))
		}
	}
}

// CloseAll disconnects all servers.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	m.mu.Unlock()
	for _, name := range names {
		if err := m.Disconnect(name); err != nil {
			slog.Debug("mcp: close error", slog.String("name", name), slog.Any("err", err))
		}
	}
}

// GetClient returns the client for a named server.
func (m *Manager) GetClient(name string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.clients[name]
	return c, ok
}

// AllTools returns a flat slice of NamespacedTool for every connected server.
func (m *Manager) AllTools() []NamespacedTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var tools []NamespacedTool
	for serverName, c := range m.clients {
		if c.ConnectionState() != StateConnected {
			continue
		}
		for _, t := range c.Tools() {
			tools = append(tools, NamespacedTool{ServerName: serverName, Tool: t})
		}
	}
	return tools
}

// AllToolsCapped returns all tools with descriptions capped to MaxMCPDescriptionLength.
func (m *Manager) AllToolsCapped() []NamespacedTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var tools []NamespacedTool
	for serverName, c := range m.clients {
		if c.ConnectionState() != StateConnected {
			continue
		}
		for _, t := range c.ToolsCapped() {
			tools = append(tools, NamespacedTool{ServerName: serverName, Tool: t})
		}
	}
	return tools
}

// AllResources returns all cached resources across connected servers.
func (m *Manager) AllResources() []ServerResource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var resources []ServerResource
	for serverName, c := range m.clients {
		if c.ConnectionState() != StateConnected {
			continue
		}
		for _, r := range c.Resources() {
			resources = append(resources, ServerResource{
				MCPResource: r,
				Server:      serverName,
			})
		}
	}
	return resources
}

// CallTool routes a tool call to the correct server.
// toolName must be in the form "serverName/toolName" or just "toolName" if
// serverName is provided separately.
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, args []byte) (*CallToolResult, error) {
	m.mu.RLock()
	c, ok := m.clients[serverName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("mcp manager: server %q not connected", serverName)
	}
	if c.ConnectionState() != StateConnected {
		return nil, fmt.Errorf("mcp manager: server %q not in connected state (state=%s)", serverName, c.ConnectionState())
	}
	return c.CallTool(ctx, toolName, args)
}

// CallToolByQualifiedName resolves an "mcp__server__tool" name and invokes it.
func (m *Manager) CallToolByQualifiedName(ctx context.Context, qualifiedName string, args json.RawMessage) (*CallToolResult, error) {
	info := McpInfoFromString(qualifiedName)
	if info == nil {
		return nil, fmt.Errorf("mcp manager: invalid qualified tool name %q", qualifiedName)
	}
	return m.CallTool(ctx, info.ServerName, info.ToolName, args)
}

// ReconnectServer attempts to reconnect a single named server.
func (m *Manager) ReconnectServer(ctx context.Context, name string) error {
	m.mu.RLock()
	c, ok := m.clients[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("mcp manager: server %q not found", name)
	}
	return c.Reconnect(ctx)
}

// ConnectionStates returns a snapshot of all server connection states.
func (m *Manager) ConnectionStates() []MCPServerConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var states []MCPServerConnection
	for _, c := range m.clients {
		states = append(states, c.ConnectionInfo())
	}
	return states
}

// GetConnectionInfo returns detailed connection info for a named server.
func (m *Manager) GetConnectionInfo(name string) (MCPServerConnection, bool) {
	m.mu.RLock()
	c, ok := m.clients[name]
	m.mu.RUnlock()
	if !ok {
		return MCPServerConnection{}, false
	}
	return c.ConnectionInfo(), true
}

// ServerCount returns the total number of registered servers.
func (m *Manager) ServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// ConnectedServerCount returns the number of connected servers.
func (m *Manager) ConnectedServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, c := range m.clients {
		if c.ConnectionState() == StateConnected {
			n++
		}
	}
	return n
}

// ConnectAllWithConcurrency connects all non-disabled servers with a concurrency limit.
// batchSize controls how many servers connect in parallel at once.
func (m *Manager) ConnectAllWithConcurrency(ctx context.Context, cfg GlobalMCPConfig, batchSize int) {
	if batchSize <= 0 {
		batchSize = 3
	}

	servers := cfg.Active()
	sem := make(chan struct{}, batchSize)
	var wg sync.WaitGroup

	for _, srv := range servers {
		wg.Add(1)
		go func(s ServerConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := m.Connect(ctx, s); err != nil {
				slog.Warn("mcp: failed to connect server",
					slog.String("name", s.Name),
					slog.Any("err", err))
			}
		}(srv)
	}
	wg.Wait()
}

// BuildCLIState builds a full MCPCliState snapshot for CLI exchange.
func (m *Manager) BuildCLIState() MCPCliState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state := MCPCliState{
		Clients:         make([]SerializedClient, 0, len(m.clients)),
		Configs:         make(map[string]ServerConfig, len(m.clients)),
		Tools:           make([]SerializedTool, 0),
		Resources:       make(map[string][]ServerResource),
		NormalizedNames: make(map[string]string),
	}

	for name, c := range m.clients {
		info := c.ConnectionInfo()
		state.Clients = append(state.Clients, SerializedClient{
			Name:         name,
			Type:         info.State,
			Capabilities: info.Capabilities,
		})
		state.Configs[name] = c.Config()

		normalized := NormalizeNameForMCP(name)
		if normalized != name {
			state.NormalizedNames[normalized] = name
		}

		if info.State == StateConnected {
			for _, t := range c.ToolsCapped() {
				state.Tools = append(state.Tools, SerializedTool{
					Name:             BuildMcpToolName(name, t.Name),
					Description:      t.Description,
					InputJSONSchema:  t.InputSchema,
					IsMcp:            true,
					OriginalToolName: t.Name,
				})
			}
			for _, r := range c.Resources() {
				state.Resources[name] = append(state.Resources[name], ServerResource{
					MCPResource: r,
					Server:      name,
				})
			}
		}
	}

	return state
}

// NamespacedTool pairs a tool with its originating server.
type NamespacedTool struct {
	ServerName string
	Tool       MCPTool
}

// QualifiedName returns "mcp__serverName__toolName".
func (n NamespacedTool) QualifiedName() string {
	return BuildMcpToolName(n.ServerName, n.Tool.Name)
}

// SlashName returns "serverName/toolName".
func (n NamespacedTool) SlashName() string {
	return n.ServerName + "/" + n.Tool.Name
}
