package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// Agent MCP server initialization aligned with claude-code-main's
// initializeAgentMcpServers in runAgent.ts.
//
// Each agent definition can reference MCP servers by name (shared with parent)
// or define inline MCP servers (agent-specific, cleaned up on agent exit).

// McpToolProvider is an interface for connecting to MCP servers and fetching tools.
// Implemented by the MCP client package to avoid import cycles.
type McpToolProvider interface {
	// ConnectAndFetchTools connects to an MCP server and returns its tools.
	ConnectAndFetchTools(ctx context.Context, spec AgentMcpServerSpec) ([]engine.Tool, CleanupFunc, error)
	// FindByName looks up an existing MCP connection by name.
	FindByName(name string) ([]engine.Tool, bool)
}

// CleanupFunc is called to disconnect an MCP server.
type CleanupFunc func() error

// AgentMcpResult holds the tools and cleanup functions from MCP initialization.
type AgentMcpResult struct {
	// Tools are all MCP-provided tools available to the agent.
	Tools []engine.Tool
	// Cleanup releases agent-specific MCP connections (not shared ones).
	Cleanup func()
}

// InitializeAgentMcpServers connects to agent-specific MCP servers and returns
// their tools. Shared (by-name) servers reuse parent connections; inline servers
// get new connections that are cleaned up when the agent finishes.
func InitializeAgentMcpServers(
	ctx context.Context,
	def *AgentDefinition,
	mcpProvider McpToolProvider,
) (*AgentMcpResult, error) {
	result := &AgentMcpResult{
		Cleanup: func() {},
	}

	if def == nil || len(def.McpServers) == 0 || mcpProvider == nil {
		return result, nil
	}

	// Skip MCP for non-admin-trusted agents when MCP is restricted to plugin-only.
	// (Aligned with initializeAgentMcpServers in runAgent.ts)

	var allTools []engine.Tool
	var cleanups []CleanupFunc
	var mu sync.Mutex

	for _, spec := range def.McpServers {
		// If spec has only a Name (no Command/URL), it's a reference to an existing server.
		if spec.Command == "" && spec.URL == "" && spec.Name != "" {
			tools, found := mcpProvider.FindByName(spec.Name)
			if !found {
				slog.Warn("agent mcp: server not found",
					slog.String("agent_type", def.AgentType),
					slog.String("server_name", spec.Name),
				)
				continue
			}
			mu.Lock()
			allTools = append(allTools, tools...)
			mu.Unlock()
			continue
		}

		// Inline definition — create new connection.
		tools, cleanup, err := mcpProvider.ConnectAndFetchTools(ctx, spec)
		if err != nil {
			slog.Warn("agent mcp: connection failed",
				slog.String("agent_type", def.AgentType),
				slog.String("server_name", spec.Name),
				slog.Any("err", err),
			)
			continue
		}

		mu.Lock()
		allTools = append(allTools, tools...)
		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
		mu.Unlock()

		slog.Info("agent mcp: connected",
			slog.String("agent_type", def.AgentType),
			slog.String("server_name", spec.Name),
			slog.Int("tools", len(tools)),
		)
	}

	result.Tools = allTools
	result.Cleanup = func() {
		for _, fn := range cleanups {
			if err := fn(); err != nil {
				slog.Warn("agent mcp: cleanup error",
					slog.String("agent_type", def.AgentType),
					slog.Any("err", err),
				)
			}
		}
	}

	return result, nil
}

// CheckRequiredMcpServers verifies that all required MCP servers for an agent
// are available. Returns an error if any required server is missing.
func CheckRequiredMcpServers(def *AgentDefinition, mcpProvider McpToolProvider) error {
	if def == nil || len(def.RequiredMcpServers) == 0 || mcpProvider == nil {
		return nil
	}

	var missing []string
	for _, name := range def.RequiredMcpServers {
		if _, found := mcpProvider.FindByName(name); !found {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("agent %q requires MCP servers not available: %v", def.AgentType, missing)
	}
	return nil
}
