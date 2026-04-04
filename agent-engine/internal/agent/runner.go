package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/engine"
)

// AgentRunner is the core agent execution lifecycle manager.
// Aligned with claude-code-main's runAgent.ts.
//
// It orchestrates:
//  1. Agent ID generation and definition resolution
//  2. Hook execution (SubagentStart)
//  3. MCP server initialization for agent-specific servers
//  4. Tool filtering via ToolFilter
//  5. Engine creation and configuration
//  6. Notification injection into query loop
//  7. Message loop execution and output collection
//  8. Hook execution (SubagentEnd)
//  9. MCP server cleanup
type AgentRunner struct {
	// caller is the LLM provider for creating engines.
	caller engine.ModelCaller
	// loader provides agent definitions.
	loader *AgentLoader
	// taskFramework registers and tracks agent tasks.
	taskFramework *TaskFramework
	// taskManager tracks agent task lifecycle.
	taskManager *TaskManager
	// worktreeManager manages git worktrees for isolation.
	worktreeManager *WorktreeManager
	// diskOutput manages async agent disk output.
	diskOutput *DiskOutput
	// mcpProvider connects to agent-specific MCP servers.
	mcpProvider McpToolProvider
	// progressRegistry tracks progress of all running agents.
	progressRegistry *ProgressRegistry
	// memoryManager handles agent memory load/save.
	memoryManager *AgentMemoryManager

	// allToolNames is the full list of tool names in the registry.
	allToolNames []string
	// allTools is the full list of tool implementations.
	allTools []engine.Tool
}

// AgentRunnerConfig configures the AgentRunner.
type AgentRunnerConfig struct {
	Caller           engine.ModelCaller
	Loader           *AgentLoader
	TaskFramework    *TaskFramework
	TaskManager      *TaskManager
	WorktreeManager  *WorktreeManager
	DiskOutput       *DiskOutput
	McpProvider      McpToolProvider
	ProgressRegistry *ProgressRegistry
	MemoryManager    *AgentMemoryManager
	AllTools         []engine.Tool
}

// NewAgentRunner creates a new AgentRunner.
func NewAgentRunner(cfg AgentRunnerConfig) *AgentRunner {
	names := make([]string, len(cfg.AllTools))
	for i, t := range cfg.AllTools {
		names[i] = t.Name()
	}
	pr := cfg.ProgressRegistry
	if pr == nil {
		pr = NewProgressRegistry()
	}
	return &AgentRunner{
		caller:           cfg.Caller,
		loader:           cfg.Loader,
		taskFramework:    cfg.TaskFramework,
		taskManager:      cfg.TaskManager,
		worktreeManager:  cfg.WorktreeManager,
		diskOutput:       cfg.DiskOutput,
		mcpProvider:      cfg.McpProvider,
		progressRegistry: pr,
		memoryManager:    cfg.MemoryManager,
		allToolNames:     names,
		allTools:         cfg.AllTools,
	}
}

// RunAgentParams holds all parameters for a single agent run.
type RunAgentParams struct {
	// AgentDef is the resolved agent definition. If nil, a general-purpose agent is used.
	AgentDef *AgentDefinition
	// Task is the prompt/task text sent to the agent.
	Task string
	// ParentContext is the subagent context from the parent (nil for root agents).
	ParentContext *SubagentContext
	// WorkDir overrides the agent's working directory.
	WorkDir string
	// Model overrides the agent model.
	Model string
	// MaxTurns overrides the max turns.
	MaxTurns int
	// AllowedTools overrides the tool filter (from AgentTool input).
	AllowedTools []string
	// SystemPrompt overrides the system prompt.
	SystemPrompt string
	// Background indicates async execution.
	Background bool
	// IsolationMode overrides isolation setting.
	IsolationMode IsolationMode
	// TeamName sets the team context.
	TeamName string
	// ParentMessages is the parent's conversation for fork subagents.
	ParentMessages []*engine.Message
	// IsFork indicates this is a fork subagent (prompt cache sharing).
	IsFork bool
	// PermissionMode overrides permission mode.
	PermissionMode string
	// ExistingAgentID reuses an existing agent ID (resume scenario).
	ExistingAgentID string
	// IsInProcessTeammate indicates this agent runs as an in-process teammate.
	IsInProcessTeammate bool
	// IsCoordinator indicates this agent runs in coordinator mode.
	IsCoordinator bool
	// NotificationQueue receives notifications from child agents.
	NotificationQueue *NotificationQueue
	// Description is a short summary of what the agent does.
	Description string
}

// AgentRunResult holds the outcome of a completed agent run.
type AgentRunResult struct {
	AgentID    string
	Output     string
	Error      error
	Status     AgentStatus
	Duration   time.Duration
	TurnCount  int
	TokenUsage *engine.UsageStats
}

// RunAgent executes the full agent lifecycle.
// This is the Go equivalent of runAgent.ts's default export.
func (r *AgentRunner) RunAgent(ctx context.Context, params RunAgentParams) *AgentRunResult {
	startTime := time.Now()

	// 1. Generate or reuse agent ID.
	agentID := params.ExistingAgentID
	if agentID == "" {
		agentID = uuid.New().String()
	}

	// 2. Resolve agent definition.
	agentDef := params.AgentDef
	if agentDef == nil {
		agentDef = &GeneralPurposeAgent
	}

	// Apply overrides from params.
	effectiveDef := r.resolveEffectiveDef(agentDef, params)
	effectiveDef.AgentID = agentID

	// 3. Assign color.
	if effectiveDef.Color == "" {
		effectiveDef.Color = AssignColor()
	}

	slog.Info("agent: starting",
		slog.String("agent_id", agentID),
		slog.String("type", effectiveDef.AgentType),
		slog.String("model", effectiveDef.Model),
		slog.Bool("background", effectiveDef.Background),
		slog.String("isolation", string(effectiveDef.Isolation)),
	)

	// 4. Execute SubagentStart hooks.
	hookCtx := &HookContext{
		AgentID:   agentID,
		AgentType: effectiveDef.AgentType,
		ParentID:  effectiveDef.ParentID,
		TeamName:  effectiveDef.TeamName,
		WorkDir:   effectiveDef.WorkDir,
		Task:      params.Task,
		IsAsync:   effectiveDef.Background,
		IsFork:    params.IsFork,
	}
	ExecuteHooks(ctx, HookSubagentStart, &effectiveDef, hookCtx)

	// 5. Register task in framework.
	if r.taskFramework != nil {
		r.taskFramework.Register(effectiveDef)
	}
	if r.taskManager != nil {
		r.taskManager.Create(effectiveDef)
		_ = r.taskManager.MarkRunning(agentID)
	}

	// 6. Register progress tracker.
	progressTracker := r.progressRegistry.Register(agentID)
	progressTracker.SetMaxTurns(effectiveDef.MaxTurns)
	defer r.progressRegistry.Remove(agentID)

	// 7. Initialize agent-specific MCP servers.
	var mcpCleanup func()
	var mcpTools []engine.Tool
	if len(effectiveDef.McpServers) > 0 && r.mcpProvider != nil {
		mcpResult, err := InitializeAgentMcpServers(ctx, &effectiveDef, r.mcpProvider)
		if err != nil {
			slog.Warn("agent: mcp init failed",
				slog.String("agent_id", agentID),
				slog.Any("err", err))
		} else {
			mcpTools = mcpResult.Tools
			mcpCleanup = mcpResult.Cleanup
		}
	}
	defer func() {
		if mcpCleanup != nil {
			mcpCleanup()
		}
	}()

	// 8. Filter tools for this agent.
	filteredToolNames := FilterToolsForAgent(
		r.allToolNames,
		&effectiveDef,
		effectiveDef.Background,
		params.IsInProcessTeammate,
		params.IsCoordinator,
	)

	// Apply additional AllowedTools from params (intersection).
	if len(params.AllowedTools) > 0 {
		allowed := make(map[string]bool)
		for _, t := range params.AllowedTools {
			allowed[t] = true
		}
		var intersected []string
		for _, t := range filteredToolNames {
			if allowed[t] {
				intersected = append(intersected, t)
			}
		}
		filteredToolNames = intersected
	}

	// Resolve tool implementations (registry tools + MCP tools).
	tools := r.resolveTools(filteredToolNames)
	tools = append(tools, mcpTools...)

	// 9. Handle worktree isolation.
	workDir := effectiveDef.WorkDir
	var worktreePath string
	if effectiveDef.Isolation == IsolationWorktree && r.worktreeManager != nil {
		var err error
		worktreePath, err = r.worktreeManager.CreateWorktree(agentID, workDir)
		if err != nil {
			slog.Warn("agent: worktree creation failed, using parent workdir",
				slog.String("agent_id", agentID),
				slog.Any("err", err))
		} else {
			workDir = worktreePath
		}
	}

	// 10. Build system prompt (dynamic or static).
	systemPrompt := effectiveDef.SystemPrompt
	if effectiveDef.GetSystemPrompt != nil {
		systemPrompt = effectiveDef.GetSystemPrompt(AgentPromptContext{
			ToolNames: filteredToolNames,
			Model:     effectiveDef.Model,
			WorkDir:   workDir,
			TeamName:  effectiveDef.TeamName,
			IsAsync:   effectiveDef.Background,
			IsFork:    params.IsFork,
		})
	}

	// Append memory prompt if memory is configured.
	if r.memoryManager != nil && effectiveDef.Memory != "" {
		memoryPrompt := r.memoryManager.BuildMemoryPrompt(&effectiveDef)
		if memoryPrompt != "" {
			systemPrompt += memoryPrompt
		}
	}

	// 11. Build engine config.
	engineCfg := engine.EngineConfig{
		WorkDir:            workDir,
		SessionID:          agentID,
		MaxTokens:          8192,
		Model:              effectiveDef.Model,
		CustomSystemPrompt: systemPrompt,
		PermissionMode:     effectiveDef.PermissionMode,
		NonInteractive:     true, // agents don't prompt users
		EffortValue:        effectiveDef.Effort,
	}

	// 12. Create engine.
	eng, err := engine.New(engineCfg, r.caller, tools)
	if err != nil {
		r.finalizeTask(agentID, "", err, worktreePath)
		return &AgentRunResult{
			AgentID:  agentID,
			Error:    err,
			Status:   AgentStatusFailed,
			Duration: time.Since(startTime),
		}
	}

	// 13. Build initial messages for fork subagent.
	var initialMessages []*engine.Message
	if params.IsFork && len(params.ParentMessages) > 0 {
		initialMessages = params.ParentMessages
	}

	// 14. Create notification queue for this agent.
	notifQueue := params.NotificationQueue
	if notifQueue == nil {
		notifQueue = NewNotificationQueue(64)
	}

	// 15. Execute the query loop.
	result := r.executeLoop(ctx, eng, params.Task, effectiveDef, initialMessages, progressTracker, notifQueue)
	result.AgentID = agentID
	result.Duration = time.Since(startTime)

	// 16. Execute SubagentEnd hooks.
	hookCtx.Status = string(result.Status)
	hookCtx.Output = result.Output
	hookCtx.TurnCount = result.TurnCount
	ExecuteHooks(ctx, HookSubagentEnd, &effectiveDef, hookCtx)

	// 17. Cleanup worktree if created.
	r.finalizeTask(agentID, result.Output, result.Error, worktreePath)

	slog.Info("agent: finished",
		slog.String("agent_id", agentID),
		slog.String("status", string(result.Status)),
		slog.Duration("duration", result.Duration),
	)

	return result
}

// resolveEffectiveDef merges the base definition with RunAgentParams overrides.
func (r *AgentRunner) resolveEffectiveDef(base *AgentDefinition, params RunAgentParams) AgentDefinition {
	def := *base // copy

	if params.WorkDir != "" {
		def.WorkDir = params.WorkDir
	}
	if params.Model != "" {
		def.Model = params.Model
	}
	if params.MaxTurns > 0 {
		def.MaxTurns = params.MaxTurns
	}
	if params.SystemPrompt != "" {
		def.SystemPrompt = params.SystemPrompt
	}
	if params.Background {
		def.Background = true
	}
	if params.IsolationMode != IsolationNone {
		def.Isolation = params.IsolationMode
	}
	if params.TeamName != "" {
		def.TeamName = params.TeamName
	}
	if params.PermissionMode != "" {
		def.PermissionMode = params.PermissionMode
	}
	if params.ParentContext != nil {
		def.ParentID = params.ParentContext.ParentAgentID
	}

	// Default max turns.
	if def.MaxTurns <= 0 {
		def.MaxTurns = 50
	}

	return def
}

// resolveTools maps tool names to tool implementations.
func (r *AgentRunner) resolveTools(names []string) []engine.Tool {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	var tools []engine.Tool
	for _, t := range r.allTools {
		if nameSet[t.Name()] {
			tools = append(tools, t)
		}
	}
	return tools
}

// executeLoop runs the main agent query loop with multi-turn support.
// Enhanced with progress tracking, notification injection, and disk output.
func (r *AgentRunner) executeLoop(
	ctx context.Context,
	eng *engine.Engine,
	task string,
	def AgentDefinition,
	initialMessages []*engine.Message,
	progressTracker *ProgressTracker,
	notifQueue *NotificationQueue,
) *AgentRunResult {
	result := &AgentRunResult{
		Status: AgentStatusRunning,
	}

	maxTurns := def.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 50
	}

	var output strings.Builder
	var totalUsage engine.UsageStats
	turnCount := 0

	// Create disk output for async agents.
	var diskOut *DiskOutput
	if def.Background && r.diskOutput != nil {
		diskOut = r.diskOutput
	}

	// Submit initial task.
	params := engine.QueryParams{
		Text:   task,
		Source: engine.QuerySourceAgent,
	}

	for turnCount < maxTurns {
		turnCount++
		progressTracker.IncrementTurn()

		// Inject pending notifications into the task text for subsequent turns.
		if turnCount > 1 && notifQueue != nil {
			notifs := notifQueue.DrainAll()
			if len(notifs) > 0 {
				var notifText strings.Builder
				for _, n := range notifs {
					notifText.WriteString(FormatSingleNotificationXML(n))
					notifText.WriteString("\n")
				}
				params.Text = notifText.String()
			}
		}

		eventCh := eng.SubmitMessage(ctx, params)

		var turnText strings.Builder
		var turnDone bool

		for ev := range eventCh {
			switch ev.Type {
			case engine.EventTextDelta:
				turnText.WriteString(ev.Text)
				// Write to disk output for async agents.
				if diskOut != nil {
					diskOut.Write(ev.Text)
				}

			case engine.EventUsage:
				if ev.Usage != nil {
					totalUsage.InputTokens += ev.Usage.InputTokens
					totalUsage.OutputTokens += ev.Usage.OutputTokens
					totalUsage.CacheCreationInputTokens += ev.Usage.CacheCreationInputTokens
					totalUsage.CacheReadInputTokens += ev.Usage.CacheReadInputTokens
					// Update token counters in task framework.
					if r.taskFramework != nil {
						_ = r.taskFramework.UpdateTokens(def.AgentID,
							ev.Usage.InputTokens, ev.Usage.OutputTokens)
					}
				}

			case engine.EventDone:
				turnDone = true

			case engine.EventError:
				result.Error = fmt.Errorf("%s", ev.Error)
				result.Status = AgentStatusFailed
				result.TurnCount = turnCount
				result.TokenUsage = &totalUsage
				result.Output = output.String()
				return result

			case engine.EventToolUse:
				// Update task activity and progress tracker.
				if ev.ToolName != "" {
					progressTracker.SetLastTool(ev.ToolName)
					if r.taskFramework != nil {
						r.taskFramework.AppendActivity(def.AgentID, fmt.Sprintf("Using %s", ev.ToolName))
					}
				}
			}
		}

		if turnText.Len() > 0 {
			text := turnText.String()
			output.WriteString(text)
			progressTracker.SetLastMessage(text)
		}

		// If the model said "done" (end_turn without tool calls), we're finished.
		if turnDone {
			break
		}

		// Check context cancellation.
		if ctx.Err() != nil {
			result.Error = ctx.Err()
			result.Status = AgentStatusCancelled
			break
		}

		// For multi-turn: the engine handles the tool execution internally
		// and re-submits. The outer loop only needs to handle explicit
		// continuation scenarios. Since the engine's query loop is self-contained
		// with multi-turn tool execution, we break after EventDone.
		break
	}

	if result.Status == AgentStatusRunning {
		result.Status = AgentStatusDone
	}

	result.Output = output.String()
	result.TurnCount = turnCount
	result.TokenUsage = &totalUsage
	return result
}

// finalizeTask marks the task complete and cleans up resources.
func (r *AgentRunner) finalizeTask(agentID, output string, err error, worktreePath string) {
	if r.taskManager != nil {
		if err != nil {
			_ = r.taskManager.MarkFailed(agentID, err.Error())
		} else {
			_ = r.taskManager.MarkDone(agentID, output)
		}
	}

	// Schedule eviction in task framework.
	if r.taskFramework != nil {
		r.taskFramework.ScheduleEviction(agentID)
	}

	// Cleanup worktree (but keep if there are changes for the user to review).
	if worktreePath != "" && r.worktreeManager != nil {
		hasChanges, _ := WorktreeHasChanges(worktreePath)
		if !hasChanges {
			// Safe to remove — no changes were made.
			_ = r.worktreeManager.RemoveWorktree(agentID, worktreePath)
		} else {
			slog.Info("agent: keeping worktree with changes",
				slog.String("agent_id", agentID),
				slog.String("path", worktreePath))
		}
	}
}

// RunAgentSync is a convenience wrapper that runs an agent synchronously
// and returns the output string.
func (r *AgentRunner) RunAgentSync(ctx context.Context, params RunAgentParams) (string, error) {
	result := r.RunAgent(ctx, params)
	if result.Error != nil {
		return "", result.Error
	}
	return result.Output, nil
}
