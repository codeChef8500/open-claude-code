package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// AgentConfig configures a sub-agent spawned by the coordinator.
type AgentConfig struct {
	AgentID      string
	Task         string
	WorkDir      string
	MaxTurns     int
	AllowedTools []string
	SystemPrompt string
	ParentID     string
}

// AgentResult holds the outcome of a completed sub-agent run.
type AgentResult struct {
	AgentID string
	Output  string
	Error   error
}

// Coordinator manages a pool of sub-agents spawned by the Task tool.
type Coordinator struct {
	mu       sync.RWMutex
	agents   map[string]*runningAgent
	prov     provider.Provider
	registry *tool.Registry
}

type runningAgent struct {
	id     string
	cancel context.CancelFunc
	doneCh chan AgentResult
}

// NewCoordinator creates a new agent coordinator.
func NewCoordinator(prov provider.Provider, registry *tool.Registry) *Coordinator {
	return &Coordinator{
		agents:   make(map[string]*runningAgent),
		prov:     prov,
		registry: registry,
	}
}

// SpawnAgent starts a sub-agent in the background and returns its ID.
// The result can be retrieved via WaitAgent.
func (c *Coordinator) SpawnAgent(ctx context.Context, cfg AgentConfig) (string, error) {
	if cfg.AgentID == "" {
		cfg.AgentID = uuid.New().String()
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 50
	}
	if cfg.WorkDir == "" {
		return "", fmt.Errorf("agent WorkDir must not be empty")
	}

	agentCtx, cancel := context.WithCancel(ctx)
	doneCh := make(chan AgentResult, 1)

	ra := &runningAgent{id: cfg.AgentID, cancel: cancel, doneCh: doneCh}
	c.mu.Lock()
	c.agents[cfg.AgentID] = ra
	c.mu.Unlock()

	go func() {
		result := c.runAgent(agentCtx, cfg)
		doneCh <- result
		c.mu.Lock()
		delete(c.agents, cfg.AgentID)
		c.mu.Unlock()
	}()

	return cfg.AgentID, nil
}

// WaitAgent blocks until the agent with agentID completes or ctx is cancelled.
func (c *Coordinator) WaitAgent(ctx context.Context, agentID string) (AgentResult, error) {
	c.mu.RLock()
	ra, ok := c.agents[agentID]
	c.mu.RUnlock()
	if !ok {
		return AgentResult{}, fmt.Errorf("agent %q not found", agentID)
	}

	select {
	case result := <-ra.doneCh:
		return result, nil
	case <-ctx.Done():
		return AgentResult{}, ctx.Err()
	}
}

// CancelAgent sends cancellation to a running agent.
func (c *Coordinator) CancelAgent(agentID string) {
	c.mu.RLock()
	ra, ok := c.agents[agentID]
	c.mu.RUnlock()
	if ok {
		ra.cancel()
	}
}

// ActiveAgents returns IDs of all currently running agents.
func (c *Coordinator) ActiveAgents() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make([]string, 0, len(c.agents))
	for id := range c.agents {
		ids = append(ids, id)
	}
	return ids
}

// runAgent executes a sub-agent task using the engine's query loop.
func (c *Coordinator) runAgent(ctx context.Context, cfg AgentConfig) AgentResult {
	// Filter allowed tools.
	var tools []tool.Tool
	if len(cfg.AllowedTools) > 0 {
		allowSet := make(map[string]bool)
		for _, n := range cfg.AllowedTools {
			allowSet[n] = true
		}
		for _, t := range c.registry.All() {
			if allowSet[t.Name()] {
				tools = append(tools, t)
			}
		}
	} else {
		tools = c.registry.All()
	}

	engineCfg := engine.EngineConfig{
		Provider:           c.prov.Name(),
		WorkDir:            cfg.WorkDir,
		SessionID:          cfg.AgentID,
		MaxTokens:          8192,
		CustomSystemPrompt: cfg.SystemPrompt,
	}

	eng, err := engine.New(engineCfg, c.prov, tools)
	if err != nil {
		return AgentResult{AgentID: cfg.AgentID, Error: err}
	}

	params := engine.QueryParams{Text: cfg.Task}
	eventCh := eng.SubmitMessage(ctx, params)

	var output string
	for ev := range eventCh {
		switch ev.Type {
		case engine.EventTextDelta:
			output += ev.Text
		case engine.EventError:
			return AgentResult{AgentID: cfg.AgentID, Error: fmt.Errorf(ev.Error)}
		}
	}

	return AgentResult{AgentID: cfg.AgentID, Output: output}
}
