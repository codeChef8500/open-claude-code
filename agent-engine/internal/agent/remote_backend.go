package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Remote backend aligned with claude-code-main's remoteAgent.ts.
//
// The remote backend provides container-level isolation for agents.
// This is a stub implementation — the actual container orchestration
// (Docker, Kubernetes, etc.) will be implemented based on deployment target.
//
// Architecture:
//   - Each remote agent runs in an isolated container
//   - Communication via gRPC or HTTP bridge
//   - File sync via volume mounts or rsync
//   - Lifecycle managed by the orchestrator

// RemoteAgentState tracks the state of a remote agent.
type RemoteAgentState string

const (
	RemoteStateProvisioning RemoteAgentState = "provisioning"
	RemoteStateRunning      RemoteAgentState = "running"
	RemoteStateStopping     RemoteAgentState = "stopping"
	RemoteStateStopped      RemoteAgentState = "stopped"
	RemoteStateFailed       RemoteAgentState = "failed"
)

// RemoteAgent represents an agent running in a remote container.
type RemoteAgent struct {
	mu sync.RWMutex

	AgentID      string
	ContainerID  string
	Endpoint     string // gRPC or HTTP endpoint
	State        RemoteAgentState
	StartedAt    time.Time
	StoppedAt    time.Time
	WorkDir      string
	Error        error

	// Resource limits.
	MemoryLimitMB int
	CPULimit      float64
	TimeoutSec    int
}

// RemoteBackend manages remote container-based agents.
type RemoteBackend struct {
	mu     sync.RWMutex
	agents map[string]*RemoteAgent
	cfg    RemoteBackendConfig
}

// RemoteBackendConfig configures the remote backend.
type RemoteBackendConfig struct {
	// Image is the container image to use for agents.
	Image string
	// Registry is the container registry URL.
	Registry string
	// DefaultMemoryMB is the default memory limit per agent.
	DefaultMemoryMB int
	// DefaultCPU is the default CPU limit per agent.
	DefaultCPU float64
	// DefaultTimeoutSec is the default timeout per agent.
	DefaultTimeoutSec int
	// Network is the Docker/container network name.
	Network string
	// SyncMethod is how files are synced: "volume", "rsync", "none".
	SyncMethod string
}

// NewRemoteBackend creates a new remote backend.
func NewRemoteBackend(cfg RemoteBackendConfig) *RemoteBackend {
	if cfg.DefaultMemoryMB <= 0 {
		cfg.DefaultMemoryMB = 2048
	}
	if cfg.DefaultCPU <= 0 {
		cfg.DefaultCPU = 1.0
	}
	if cfg.DefaultTimeoutSec <= 0 {
		cfg.DefaultTimeoutSec = 3600
	}
	if cfg.SyncMethod == "" {
		cfg.SyncMethod = "volume"
	}

	return &RemoteBackend{
		agents: make(map[string]*RemoteAgent),
		cfg:    cfg,
	}
}

// SpawnAgent provisions and starts a remote agent container.
// This is a stub — actual container orchestration is not yet implemented.
func (rb *RemoteBackend) SpawnAgent(ctx context.Context, agentID, task, workDir string) (*RemoteAgent, error) {
	agent := &RemoteAgent{
		AgentID:       agentID,
		State:         RemoteStateProvisioning,
		StartedAt:     time.Now(),
		WorkDir:       workDir,
		MemoryLimitMB: rb.cfg.DefaultMemoryMB,
		CPULimit:      rb.cfg.DefaultCPU,
		TimeoutSec:    rb.cfg.DefaultTimeoutSec,
	}

	rb.mu.Lock()
	rb.agents[agentID] = agent
	rb.mu.Unlock()

	// TODO: Implement actual container provisioning.
	// For now, this is a stub that logs the intent.
	slog.Warn("remote_backend: container provisioning not yet implemented",
		slog.String("agent_id", agentID),
		slog.String("image", rb.cfg.Image),
	)

	agent.mu.Lock()
	agent.State = RemoteStateFailed
	agent.Error = fmt.Errorf("remote backend not yet implemented — use worktree isolation as fallback")
	agent.mu.Unlock()

	return agent, agent.Error
}

// StopAgent stops a remote agent container.
func (rb *RemoteBackend) StopAgent(agentID string) error {
	rb.mu.RLock()
	agent, ok := rb.agents[agentID]
	rb.mu.RUnlock()

	if !ok {
		return fmt.Errorf("remote agent not found: %s", agentID)
	}

	agent.mu.Lock()
	agent.State = RemoteStateStopped
	agent.StoppedAt = time.Now()
	agent.mu.Unlock()

	// TODO: Implement container stop/kill.
	slog.Info("remote_backend: stopped agent (stub)", slog.String("agent_id", agentID))
	return nil
}

// GetAgent returns a remote agent by ID.
func (rb *RemoteBackend) GetAgent(agentID string) (*RemoteAgent, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	a, ok := rb.agents[agentID]
	return a, ok
}

// AllAgents returns all remote agents.
func (rb *RemoteBackend) AllAgents() []*RemoteAgent {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	result := make([]*RemoteAgent, 0, len(rb.agents))
	for _, a := range rb.agents {
		result = append(result, a)
	}
	return result
}

// StopAll stops all remote agents.
func (rb *RemoteBackend) StopAll() {
	rb.mu.RLock()
	ids := make([]string, 0, len(rb.agents))
	for id := range rb.agents {
		ids = append(ids, id)
	}
	rb.mu.RUnlock()

	for _, id := range ids {
		_ = rb.StopAgent(id)
	}
}

// IsAvailable checks if the remote backend infrastructure is accessible.
// Returns false since this is a stub implementation.
func (rb *RemoteBackend) IsAvailable() bool {
	// TODO: Check Docker/Kubernetes connectivity.
	return false
}

// GetEndpoint returns the communication endpoint for a remote agent.
func (rb *RemoteBackend) GetEndpoint(agentID string) (string, error) {
	rb.mu.RLock()
	agent, ok := rb.agents[agentID]
	rb.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("remote agent not found: %s", agentID)
	}

	agent.mu.RLock()
	defer agent.mu.RUnlock()

	if agent.Endpoint == "" {
		return "", fmt.Errorf("remote agent %s has no endpoint", agentID)
	}
	return agent.Endpoint, nil
}
