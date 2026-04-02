package mcp

import (
	"context"
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
		for _, t := range c.Tools() {
			tools = append(tools, NamespacedTool{ServerName: serverName, Tool: t})
		}
	}
	return tools
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
	return c.CallTool(ctx, toolName, args)
}

// NamespacedTool pairs a tool with its originating server.
type NamespacedTool struct {
	ServerName string
	Tool       MCPTool
}

// QualifiedName returns "serverName/toolName".
func (n NamespacedTool) QualifiedName() string {
	return n.ServerName + "/" + n.Tool.Name
}
