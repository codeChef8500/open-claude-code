package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// InProcess teammate implementation aligned with claude-code-main's
// inProcessTeammate.ts.
//
// An InProcess teammate is a goroutine-based agent that:
//   - Runs concurrently within the same process
//   - Has its own mailbox for receiving messages
//   - Can communicate via the message bus
//   - Shares the agent runner infrastructure
//   - Has restricted tool access (InProcessTeammateAllowedTools)

// TeammateState represents the lifecycle state of an in-process teammate.
type TeammateState string

const (
	TeammateStateIdle     TeammateState = "idle"
	TeammateStateRunning  TeammateState = "running"
	TeammateStateStopping TeammateState = "stopping"
	TeammateStateStopped  TeammateState = "stopped"
	TeammateStateFailed   TeammateState = "failed"
)

// InProcessTeammate is a goroutine-based teammate agent.
type InProcessTeammate struct {
	mu sync.RWMutex

	// Identity.
	AgentID    string
	Definition AgentDefinition
	TeamName   string

	// State.
	State     TeammateState
	StartedAt time.Time
	StoppedAt time.Time
	Error     error

	// Communication.
	mailbox  *Mailbox
	registry *MailboxRegistry
	bus      *MessageBus

	// Execution.
	runner *AgentRunner
	cancel context.CancelFunc
	done   chan struct{}

	// Configuration.
	pollInterval  time.Duration // how often to check mailbox
	parentContext *SubagentContext
}

// InProcessTeammateConfig configures an in-process teammate.
type InProcessTeammateConfig struct {
	AgentID       string
	Definition    AgentDefinition
	TeamName      string
	Runner        *AgentRunner
	Registry      *MailboxRegistry
	Bus           *MessageBus
	PollInterval  time.Duration
	ParentContext *SubagentContext
}

// NewInProcessTeammate creates a new in-process teammate (not yet started).
func NewInProcessTeammate(cfg InProcessTeammateConfig) *InProcessTeammate {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}

	// Get or create mailbox.
	var mailbox *Mailbox
	if cfg.Registry != nil {
		mailbox = cfg.Registry.GetOrCreate(cfg.AgentID)
	}

	return &InProcessTeammate{
		AgentID:       cfg.AgentID,
		Definition:    cfg.Definition,
		TeamName:      cfg.TeamName,
		State:         TeammateStateIdle,
		mailbox:       mailbox,
		registry:      cfg.Registry,
		bus:           cfg.Bus,
		runner:        cfg.Runner,
		done:          make(chan struct{}),
		pollInterval:  cfg.PollInterval,
		parentContext: cfg.ParentContext,
	}
}

// Start launches the teammate's background goroutine.
// The teammate will poll its mailbox and process incoming messages.
func (t *InProcessTeammate) Start(parentCtx context.Context) error {
	t.mu.Lock()
	if t.State == TeammateStateRunning {
		t.mu.Unlock()
		return fmt.Errorf("teammate %s is already running", t.AgentID)
	}

	ctx, cancel := context.WithCancel(parentCtx)
	t.cancel = cancel
	t.State = TeammateStateRunning
	t.StartedAt = time.Now()
	t.mu.Unlock()

	// Subscribe to message bus.
	if t.bus != nil {
		_, _ = t.bus.Subscribe(t.AgentID, 64)
	}

	go t.runLoop(ctx)

	slog.Info("teammate: started",
		slog.String("agent_id", t.AgentID),
		slog.String("team", t.TeamName),
	)

	return nil
}

// Stop gracefully shuts down the teammate.
func (t *InProcessTeammate) Stop() {
	t.mu.Lock()
	if t.State != TeammateStateRunning {
		t.mu.Unlock()
		return
	}
	t.State = TeammateStateStopping
	t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
	}

	// Wait for the goroutine to exit.
	select {
	case <-t.done:
	case <-time.After(30 * time.Second):
		slog.Warn("teammate: stop timeout", slog.String("agent_id", t.AgentID))
	}

	t.mu.Lock()
	t.State = TeammateStateStopped
	t.StoppedAt = time.Now()
	t.mu.Unlock()

	// Unsubscribe from message bus.
	if t.bus != nil {
		t.bus.Unsubscribe(t.AgentID)
	}

	// Remove mailbox.
	if t.registry != nil {
		t.registry.Remove(t.AgentID)
	}

	slog.Info("teammate: stopped", slog.String("agent_id", t.AgentID))
}

// SendMessage sends a message to this teammate's mailbox.
func (t *InProcessTeammate) SendMessage(from, text string) (string, error) {
	if t.mailbox == nil {
		return "", fmt.Errorf("teammate %s has no mailbox", t.AgentID)
	}
	return t.mailbox.Deliver(from, text, MailboxPriorityNormal, "")
}

// GetState returns the current teammate state.
func (t *InProcessTeammate) GetState() TeammateState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.State
}

// IsAlive returns true if the teammate is running.
func (t *InProcessTeammate) IsAlive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.State == TeammateStateRunning
}

// runLoop is the main processing loop for the teammate goroutine.
func (t *InProcessTeammate) runLoop(ctx context.Context) {
	defer close(t.done)

	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.processMailbox(ctx)
		}
	}
}

// processMailbox checks for and processes pending messages.
func (t *InProcessTeammate) processMailbox(ctx context.Context) {
	if t.mailbox == nil {
		return
	}

	messages := t.mailbox.Read()
	if len(messages) == 0 {
		return
	}

	for _, msg := range messages {
		if ctx.Err() != nil {
			return
		}

		slog.Debug("teammate: processing message",
			slog.String("agent_id", t.AgentID),
			slog.String("from", msg.From),
			slog.String("msg_id", msg.ID),
		)

		// Mark as processing.
		t.mailbox.MarkProcessing(msg.ID)

		// Check for shutdown request.
		if isShutdownMessage(msg.Text) {
			slog.Info("teammate: received shutdown request",
				slog.String("agent_id", t.AgentID),
			)
			t.mailbox.Ack(msg.ID)
			go t.Stop()
			return
		}

		// Process the message by running the agent.
		t.handleMessage(ctx, msg)

		// Mark as processed.
		t.mailbox.Ack(msg.ID)
	}
}

// handleMessage processes a single mailbox message by spawning a task.
func (t *InProcessTeammate) handleMessage(ctx context.Context, msg MailboxMessage) {
	if t.runner == nil {
		slog.Warn("teammate: no runner configured", slog.String("agent_id", t.AgentID))
		return
	}

	// Derive SubagentContext for the task if available.
	var childCtx *SubagentContext
	if t.parentContext != nil {
		var err error
		childCtx, err = t.parentContext.DeriveChild(t.AgentID + "-task-" + msg.ID[:8])
		if err != nil {
			slog.Warn("teammate: derive child context failed",
				slog.String("agent_id", t.AgentID), slog.Any("err", err))
		}
	}

	params := RunAgentParams{
		AgentDef:            &t.Definition,
		Task:                msg.Text,
		Background:          false, // process synchronously within the teammate goroutine
		ExistingAgentID:     t.AgentID + "-task-" + msg.ID[:8],
		IsInProcessTeammate: true,
		TeamName:            t.TeamName,
		ParentContext:       childCtx,
	}

	result := t.runner.RunAgent(ctx, params)

	// Send reply back to the sender if possible.
	if msg.From != "" && t.registry != nil {
		reply := formatTeammateReply(t.AgentID, result)
		_, err := t.registry.Send(t.AgentID, msg.From, reply, MailboxPriorityNormal, msg.ID)
		if err != nil {
			slog.Warn("teammate: failed to send reply",
				slog.String("agent_id", t.AgentID),
				slog.String("to", msg.From),
				slog.Any("err", err),
			)
		}
	}
}

// isShutdownMessage checks if a message is a shutdown request.
func isShutdownMessage(text string) bool {
	return text == "__shutdown__" || text == "__stop__"
}

// formatTeammateReply formats a teammate's task result as a reply message.
func formatTeammateReply(agentID string, result *AgentRunResult) string {
	if result.Error != nil {
		return fmt.Sprintf("[%s] Task failed: %v", truncID(agentID), result.Error)
	}
	output := result.Output
	if len(output) > 2000 {
		output = TruncateSubagentOutput(output, 2000)
	}
	return fmt.Sprintf("[%s] Task completed:\n%s", truncID(agentID), output)
}

// ────────────────────────────────────────────────────────────────────────────
// TeammateRegistry — manages all in-process teammates in a session.
// ────────────────────────────────────────────────────────────────────────────

// TeammateRegistry tracks all active in-process teammates.
type TeammateRegistry struct {
	mu        sync.RWMutex
	teammates map[string]*InProcessTeammate
}

// NewTeammateRegistry creates a new teammate registry.
func NewTeammateRegistry() *TeammateRegistry {
	return &TeammateRegistry{
		teammates: make(map[string]*InProcessTeammate),
	}
}

// Register adds a teammate to the registry.
func (r *TeammateRegistry) Register(t *InProcessTeammate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.teammates[t.AgentID] = t
}

// Get returns a teammate by ID.
func (r *TeammateRegistry) Get(agentID string) (*InProcessTeammate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.teammates[agentID]
	return t, ok
}

// Remove removes a teammate from the registry.
func (r *TeammateRegistry) Remove(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.teammates, agentID)
}

// All returns all registered teammates.
func (r *TeammateRegistry) All() []*InProcessTeammate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*InProcessTeammate, 0, len(r.teammates))
	for _, t := range r.teammates {
		result = append(result, t)
	}
	return result
}

// Active returns only running teammates.
func (r *TeammateRegistry) Active() []*InProcessTeammate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*InProcessTeammate
	for _, t := range r.teammates {
		if t.IsAlive() {
			result = append(result, t)
		}
	}
	return result
}

// FindByTeam returns all teammates belonging to a specific team.
func (r *TeammateRegistry) FindByTeam(teamName string) []*InProcessTeammate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*InProcessTeammate
	for _, t := range r.teammates {
		if t.TeamName == teamName {
			result = append(result, t)
		}
	}
	return result
}

// StopAll gracefully stops all teammates.
func (r *TeammateRegistry) StopAll() {
	r.mu.RLock()
	teammates := make([]*InProcessTeammate, 0, len(r.teammates))
	for _, t := range r.teammates {
		teammates = append(teammates, t)
	}
	r.mu.RUnlock()

	var wg sync.WaitGroup
	for _, t := range teammates {
		wg.Add(1)
		go func(tm *InProcessTeammate) {
			defer wg.Done()
			tm.Stop()
		}(t)
	}
	wg.Wait()
}

// SpawnTeammate is a convenience function to create, register, and start a teammate.
// Aligned with claude-code-main's spawnTeammate.
func SpawnTeammate(
	ctx context.Context,
	registry *TeammateRegistry,
	cfg InProcessTeammateConfig,
) (*InProcessTeammate, error) {
	// Validate.
	if cfg.AgentID == "" {
		return nil, fmt.Errorf("teammate AgentID is required")
	}
	if cfg.Runner == nil {
		return nil, fmt.Errorf("teammate Runner is required")
	}

	// Check for duplicates.
	if existing, ok := registry.Get(cfg.AgentID); ok {
		if existing.IsAlive() {
			return nil, fmt.Errorf("teammate %s is already running", cfg.AgentID)
		}
		// Remove stale entry.
		registry.Remove(cfg.AgentID)
	}

	tm := NewInProcessTeammate(cfg)
	registry.Register(tm)

	if err := tm.Start(ctx); err != nil {
		registry.Remove(cfg.AgentID)
		return nil, fmt.Errorf("start teammate: %w", err)
	}

	slog.Info("teammate: spawned",
		slog.String("agent_id", cfg.AgentID),
		slog.String("team", cfg.TeamName),
	)

	return tm, nil
}
