package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Coordinator mode aligned with claude-code-main's coordinatorMode.ts.
//
// The coordinator orchestrates multiple worker agents to complete a complex task:
//  1. Breaks tasks into independent work items
//  2. Spawns worker agents (each in its own worktree)
//  3. Monitors progress and handles failures
//  4. Synthesizes results when all workers complete
//  5. Uses a shared scratchpad directory for coordination

// CoordinatorConfig configures the coordinator mode.
type CoordinatorConfig struct {
	// MaxWorkers is the maximum number of concurrent worker agents.
	MaxWorkers int
	// ScratchpadDir is the shared directory for coordination files.
	ScratchpadDir string
	// WorkDir is the base working directory.
	WorkDir string
	// DefaultModel is the model for worker agents.
	DefaultModel string
	// MaxTurnsPerWorker is the max turns each worker gets.
	MaxTurnsPerWorker int
	// ForceAsync forces all workers to run asynchronously.
	ForceAsync bool
	// WorkerToolRestrictions limits which tools workers can use.
	WorkerToolRestrictions []string
	// InjectWorktreeNotice injects worktree awareness into worker prompts.
	InjectWorktreeNotice bool
}

// CoordinatorState tracks the state of a coordinator session.
type CoordinatorState struct {
	mu sync.RWMutex

	Config  CoordinatorConfig
	Workers map[string]*CoordinatorWorker
	Plan    *CoordinatorPlan
	Status  CoordinatorStatus
	StartAt time.Time
}

// CoordinatorStatus represents the lifecycle state of the coordinator.
type CoordinatorStatus string

const (
	CoordStatusIdle     CoordinatorStatus = "idle"
	CoordStatusPlanning CoordinatorStatus = "planning"
	CoordStatusRunning  CoordinatorStatus = "running"
	CoordStatusDone     CoordinatorStatus = "done"
	CoordStatusFailed   CoordinatorStatus = "failed"
)

// CoordinatorWorker tracks a single worker agent within the coordinator.
type CoordinatorWorker struct {
	AgentID     string
	Task        string
	Description string
	Status      AsyncAgentStatus
	Result      *AgentRunResult
	WorktreeDir string
	StartedAt   time.Time
	FinishedAt  time.Time
}

// CoordinatorPlan holds the decomposed task plan.
type CoordinatorPlan struct {
	Title       string
	Description string
	WorkItems   []WorkItem
	CreatedAt   time.Time
}

// WorkItem is a single unit of work in the coordinator plan.
type WorkItem struct {
	ID          string   `json:"id"`
	Task        string   `json:"task"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Assigned    bool     `json:"assigned"`
	AgentID     string   `json:"agent_id,omitempty"`
}

// CoordinatorMode manages the coordinator lifecycle.
type CoordinatorMode struct {
	mu    sync.RWMutex
	state *CoordinatorState

	runner       *AgentRunner
	asyncManager *AsyncLifecycleManager
	loader       *AgentLoader
}

// NewCoordinatorMode creates a coordinator mode manager.
func NewCoordinatorMode(
	runner *AgentRunner,
	asyncManager *AsyncLifecycleManager,
	loader *AgentLoader,
	cfg CoordinatorConfig,
) *CoordinatorMode {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 4
	}
	if cfg.MaxTurnsPerWorker <= 0 {
		cfg.MaxTurnsPerWorker = 100
	}

	return &CoordinatorMode{
		state: &CoordinatorState{
			Config:  cfg,
			Workers: make(map[string]*CoordinatorWorker),
			Status:  CoordStatusIdle,
			StartAt: time.Now(),
		},
		runner:       runner,
		asyncManager: asyncManager,
		loader:       loader,
	}
}

// IsActive returns whether coordinator mode is currently active.
func (cm *CoordinatorMode) IsActive() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.state.Status == CoordStatusRunning || cm.state.Status == CoordStatusPlanning
}

// SetPlan sets the coordinator's work plan.
func (cm *CoordinatorMode) SetPlan(plan *CoordinatorPlan) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.state.Plan = plan
	cm.state.Status = CoordStatusPlanning
}

// SpawnWorker launches a worker agent for a work item.
func (cm *CoordinatorMode) SpawnWorker(ctx context.Context, item WorkItem) (string, error) {
	cm.mu.Lock()
	if cm.state.Status == CoordStatusIdle {
		cm.state.Status = CoordStatusRunning
	}

	// Check worker limit.
	activeCount := 0
	for _, w := range cm.state.Workers {
		if w.Status == AsyncStatusRunning || w.Status == AsyncStatusPending {
			activeCount++
		}
	}
	if activeCount >= cm.state.Config.MaxWorkers {
		cm.mu.Unlock()
		return "", fmt.Errorf("max workers (%d) reached", cm.state.Config.MaxWorkers)
	}
	cm.mu.Unlock()

	// Build worker agent definition.
	workerDef := AgentDefinition{
		AgentType:  "coordinator-worker",
		Source:     SourceBuiltIn,
		Background: true,
		Isolation:  IsolationWorktree,
		MaxTurns:   cm.state.Config.MaxTurnsPerWorker,
		Model:      cm.state.Config.DefaultModel,
	}

	params := RunAgentParams{
		AgentDef:      &workerDef,
		Task:          item.Task,
		WorkDir:       cm.state.Config.WorkDir,
		Background:    true,
		IsolationMode: IsolationWorktree,
	}

	if cm.asyncManager == nil {
		return "", fmt.Errorf("async manager not configured for coordinator mode")
	}

	agentID, err := cm.asyncManager.Launch(ctx, params)
	if err != nil {
		return "", fmt.Errorf("spawn worker: %w", err)
	}

	cm.mu.Lock()
	cm.state.Workers[agentID] = &CoordinatorWorker{
		AgentID:     agentID,
		Task:        item.Task,
		Description: item.Description,
		Status:      AsyncStatusRunning,
		StartedAt:   time.Now(),
	}
	cm.mu.Unlock()

	slog.Info("coordinator: spawned worker",
		slog.String("agent_id", agentID),
		slog.String("task", item.Description),
	)

	return agentID, nil
}

// GetWorkerStatus returns the status of a specific worker.
func (cm *CoordinatorMode) GetWorkerStatus(agentID string) (*CoordinatorWorker, error) {
	cm.mu.RLock()
	w, ok := cm.state.Workers[agentID]
	cm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("worker not found: %s", agentID)
	}

	// Sync status from async manager.
	if cm.asyncManager != nil {
		status, err := cm.asyncManager.GetStatus(agentID)
		if err == nil {
			w.Status = status
		}
	}

	return w, nil
}

// AllWorkers returns all worker states.
func (cm *CoordinatorMode) AllWorkers() []*CoordinatorWorker {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	workers := make([]*CoordinatorWorker, 0, len(cm.state.Workers))
	for _, w := range cm.state.Workers {
		workers = append(workers, w)
	}
	return workers
}

// WaitAll waits for all workers to complete with a timeout.
func (cm *CoordinatorMode) WaitAll(timeout time.Duration) error {
	cm.mu.RLock()
	ids := make([]string, 0, len(cm.state.Workers))
	for id := range cm.state.Workers {
		ids = append(ids, id)
	}
	cm.mu.RUnlock()

	if cm.asyncManager == nil {
		return fmt.Errorf("async manager not configured")
	}

	for _, id := range ids {
		result, err := cm.asyncManager.Wait(id, timeout)
		if err != nil {
			slog.Warn("coordinator: worker wait failed",
				slog.String("agent_id", id),
				slog.Any("err", err))
			continue
		}

		cm.mu.Lock()
		if w, ok := cm.state.Workers[id]; ok {
			w.Result = result
			w.FinishedAt = time.Now()
			if result.Error != nil {
				w.Status = AsyncStatusFailed
			} else {
				w.Status = AsyncStatusDone
			}
		}
		cm.mu.Unlock()
	}

	return nil
}

// CollectResults returns the results from all completed workers.
func (cm *CoordinatorMode) CollectResults() map[string]*AgentRunResult {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	results := make(map[string]*AgentRunResult)
	for id, w := range cm.state.Workers {
		if w.Result != nil {
			results[id] = w.Result
		}
	}
	return results
}

// Finish marks the coordinator as done.
func (cm *CoordinatorMode) Finish() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	allDone := true
	anyFailed := false
	for _, w := range cm.state.Workers {
		if w.Status == AsyncStatusRunning || w.Status == AsyncStatusPending {
			allDone = false
		}
		if w.Status == AsyncStatusFailed {
			anyFailed = true
		}
	}

	if !allDone {
		cm.state.Status = CoordStatusRunning
		return
	}

	if anyFailed {
		cm.state.Status = CoordStatusFailed
	} else {
		cm.state.Status = CoordStatusDone
	}
}

// EnsureScratchpad creates the shared scratchpad directory if needed.
func (cm *CoordinatorMode) EnsureScratchpad() (string, error) {
	dir := cm.state.Config.ScratchpadDir
	if dir == "" {
		dir = filepath.Join(cm.state.Config.WorkDir, ".coordinator-scratchpad")
		cm.state.Config.ScratchpadDir = dir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create scratchpad: %w", err)
	}
	return dir, nil
}

// WriteScratchpad writes content to a named file in the scratchpad.
func (cm *CoordinatorMode) WriteScratchpad(name, content string) error {
	dir, err := cm.EnsureScratchpad()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

// ReadScratchpad reads a named file from the scratchpad.
func (cm *CoordinatorMode) ReadScratchpad(name string) (string, error) {
	dir := cm.state.Config.ScratchpadDir
	if dir == "" {
		return "", fmt.Errorf("scratchpad not initialized")
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatSummary returns a human-readable summary of the coordinator state.
func (cm *CoordinatorMode) FormatSummary() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	summary := fmt.Sprintf("Coordinator Status: %s\n", cm.state.Status)
	summary += fmt.Sprintf("Workers: %d\n\n", len(cm.state.Workers))

	for _, w := range cm.state.Workers {
		line := fmt.Sprintf("  [%s] %s - %s", w.Status, truncID(w.AgentID), w.Description)
		if w.Status == AsyncStatusDone && w.Result != nil {
			dur := w.FinishedAt.Sub(w.StartedAt).Round(time.Second)
			line += fmt.Sprintf(" (completed in %s)", dur)
		}
		summary += line + "\n"
	}

	return summary
}

// ── Coordinator System Prompt ──────────────────────────────────────────────
// Aligned with claude-code-main's coordinatorMode system prompt injection.

// CoordinatorAllowedTools defines the tools available to the coordinator agent.
// The coordinator itself does NOT run tools directly — it delegates to workers.
var CoordinatorAllowedTools = []string{
	"Task",      // spawn workers
	"Read",      // read files for planning
	"Grep",      // search for context
	"Glob",      // find files
	"TodoRead",  // read scratchpad
	"TodoWrite", // write scratchpad
}

// BuildCoordinatorSystemPrompt generates the system prompt for the coordinator agent.
// Aligned with claude-code-main's coordinatorMode system prompt.
func BuildCoordinatorSystemPrompt(cfg CoordinatorConfig, activeWorkers []*CoordinatorWorker) string {
	var sb strings.Builder

	sb.WriteString("You are operating in COORDINATOR MODE. Your role is to break down complex tasks ")
	sb.WriteString("into independent work items and delegate them to worker agents.\n\n")

	sb.WriteString("## Rules\n\n")
	sb.WriteString("1. **ALWAYS delegate work** — never perform file edits or run commands yourself.\n")
	sb.WriteString("2. **Spawn workers asynchronously** — all workers run in background with worktree isolation.\n")
	sb.WriteString("3. **Maximize parallelism** — spawn independent workers simultaneously.\n")
	sb.WriteString("4. **Provide complete context** — each worker starts fresh, brief them thoroughly.\n")
	sb.WriteString("5. **Monitor progress** — check worker status and handle failures.\n")
	sb.WriteString("6. **Synthesize results** — when all workers complete, summarize the outcome.\n\n")

	sb.WriteString(fmt.Sprintf("## Limits\n\n- Max concurrent workers: %d\n", cfg.MaxWorkers))
	sb.WriteString(fmt.Sprintf("- Max turns per worker: %d\n", cfg.MaxTurnsPerWorker))

	if cfg.ScratchpadDir != "" {
		sb.WriteString(fmt.Sprintf("- Scratchpad: %s\n", cfg.ScratchpadDir))
	}
	sb.WriteString("\n")

	// Show active workers if any.
	if len(activeWorkers) > 0 {
		sb.WriteString("## Active Workers\n\n")
		for _, w := range activeWorkers {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", w.Status, truncID(w.AgentID), w.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Worker Spawning\n\n")
	sb.WriteString("Use the Task tool with `run_in_background: true` and `isolation: \"worktree\"`.\n")
	sb.WriteString("Each worker gets its own git worktree to avoid conflicts.\n")
	sb.WriteString("Workers cannot see each other's changes until they complete and merge.\n")

	return sb.String()
}

// BuildWorkerWorktreeNotice generates the worktree notice injected into worker prompts.
// Aligned with claude-code-main's worktree notice for coordinator workers.
func BuildWorkerWorktreeNotice(worktreeDir, mainBranch string) string {
	var sb strings.Builder
	sb.WriteString("## Worktree Notice\n\n")
	sb.WriteString(fmt.Sprintf("You are working in a git worktree at: %s\n", worktreeDir))
	sb.WriteString(fmt.Sprintf("The main branch is: %s\n\n", mainBranch))
	sb.WriteString("Important:\n")
	sb.WriteString("- Only modify files within your worktree.\n")
	sb.WriteString("- Do NOT switch branches or merge.\n")
	sb.WriteString("- Your changes will be merged by the coordinator when you complete.\n")
	sb.WriteString("- If you encounter merge conflicts, document them and let the coordinator handle it.\n")
	return sb.String()
}

// SpawnWorkerForced launches a worker with forced async + worktree + tool restrictions.
// This is the coordinator-mode-specific spawn that enforces all constraints.
func (cm *CoordinatorMode) SpawnWorkerForced(ctx context.Context, item WorkItem, parentContext *SubagentContext) (string, error) {
	cm.mu.Lock()
	if cm.state.Status == CoordStatusIdle {
		cm.state.Status = CoordStatusRunning
	}

	activeCount := 0
	for _, w := range cm.state.Workers {
		if w.Status == AsyncStatusRunning || w.Status == AsyncStatusPending {
			activeCount++
		}
	}
	if activeCount >= cm.state.Config.MaxWorkers {
		cm.mu.Unlock()
		return "", fmt.Errorf("max workers (%d) reached", cm.state.Config.MaxWorkers)
	}
	cm.mu.Unlock()

	workerDef := AgentDefinition{
		AgentType:  "coordinator-worker",
		Source:     SourceBuiltIn,
		Background: true,
		Isolation:  IsolationWorktree,
		MaxTurns:   cm.state.Config.MaxTurnsPerWorker,
		Model:      cm.state.Config.DefaultModel,
	}

	// Apply tool restrictions from coordinator config.
	if len(cm.state.Config.WorkerToolRestrictions) > 0 {
		workerDef.AllowedTools = cm.state.Config.WorkerToolRestrictions
	}

	params := RunAgentParams{
		AgentDef:      &workerDef,
		Task:          item.Task,
		Description:   item.Description,
		WorkDir:       cm.state.Config.WorkDir,
		Background:    true,
		IsolationMode: IsolationWorktree,
		IsCoordinator: true,
		ParentContext: parentContext,
	}

	if cm.asyncManager == nil {
		return "", fmt.Errorf("async manager not configured for coordinator mode")
	}

	agentID, err := cm.asyncManager.Launch(ctx, params)
	if err != nil {
		return "", fmt.Errorf("spawn worker: %w", err)
	}

	cm.mu.Lock()
	cm.state.Workers[agentID] = &CoordinatorWorker{
		AgentID:     agentID,
		Task:        item.Task,
		Description: item.Description,
		Status:      AsyncStatusRunning,
		StartedAt:   time.Now(),
	}
	cm.mu.Unlock()

	slog.Info("coordinator: spawned forced-async worker",
		slog.String("agent_id", agentID),
		slog.String("task", item.Description),
	)

	return agentID, nil
}

// PollWorkers syncs worker statuses from the async manager and returns
// newly completed worker IDs.
func (cm *CoordinatorMode) PollWorkers() []string {
	if cm.asyncManager == nil {
		return nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	var completed []string
	for id, w := range cm.state.Workers {
		if w.Status == AsyncStatusDone || w.Status == AsyncStatusFailed || w.Status == AsyncStatusCancelled {
			continue
		}

		status, err := cm.asyncManager.GetStatus(id)
		if err != nil {
			continue
		}

		if status != w.Status {
			w.Status = status
			if status == AsyncStatusDone || status == AsyncStatusFailed || status == AsyncStatusCancelled {
				w.FinishedAt = time.Now()
				if result, err := cm.asyncManager.GetResult(id); err == nil {
					w.Result = result
				}
				completed = append(completed, id)
			}
		}
	}
	return completed
}

// FormatWorkerResults returns a formatted summary of all completed worker results.
func (cm *CoordinatorMode) FormatWorkerResults(maxCharsPerWorker int) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("## Worker Results\n\n")

	for _, w := range cm.state.Workers {
		sb.WriteString(fmt.Sprintf("### Worker %s: %s\n", truncID(w.AgentID), w.Description))
		sb.WriteString(fmt.Sprintf("Status: %s\n", w.Status))

		if w.Result != nil {
			output := w.Result.Output
			if maxCharsPerWorker > 0 && len(output) > maxCharsPerWorker {
				output = TruncateSubagentOutput(output, maxCharsPerWorker)
			}
			sb.WriteString(output)
			sb.WriteString("\n")
		} else if w.Status == AsyncStatusFailed {
			sb.WriteString("(failed with no output)\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
