package agentool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/agent"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// Input is the JSON input schema for the AgentTool.
// Extended to support the full 4-way decision tree aligned with
// claude-code-main's AgentToolInput.
type Input struct {
	Task            string   `json:"task"`
	Description     string   `json:"description,omitempty"`
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	MaxTurns        int      `json:"max_turns,omitempty"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	SubagentType    string   `json:"subagent_type,omitempty"`
	RunInBackground bool     `json:"run_in_background,omitempty"`
	Model           string   `json:"model,omitempty"`

	// ── Extended fields for multi-agent coordination ──────────────────
	Isolation string `json:"isolation,omitempty"` // "worktree", "remote", ""
	Teammate  bool   `json:"teammate,omitempty"`  // spawn as in-process teammate
	Fork      bool   `json:"fork,omitempty"`      // fork subagent (prompt cache sharing)

	// ── Swarm / Team fields (claude-code-main AgentToolInput) ────────
	Name     string `json:"name,omitempty"`      // teammate name for spawn
	TeamName string `json:"team_name,omitempty"` // team to spawn into
	Mode     string `json:"mode,omitempty"`      // "plan" forces plan mode
	Cwd      string `json:"cwd,omitempty"`       // working directory override
}

// Built-in subagent types and their default tool sets.
var subagentTypes = map[string][]string{
	"general": nil, // all tools
	"explore": {"Read", "Grep", "Glob", "Bash", "lsp"},
	"plan":    {"Read", "Grep", "Glob", "Bash"},
	"verify":  {"Read", "Grep", "Glob", "Bash", "PowerShell"},
}

// SubAgentRunner is the legacy callback the parent engine provides to launch a child agent.
type SubAgentRunner func(ctx context.Context, agentID, task string, input Input, uctx *tool.UseContext) (string, error)

// AgentToolConfig provides dependencies for the 4-way decision tree.
type AgentToolConfig struct {
	// Runner is the core agent runner (Phase 2).
	Runner *agent.AgentRunner
	// AsyncManager manages background agent lifecycles (Phase 3).
	AsyncManager *agent.AsyncLifecycleManager
	// Loader provides agent definitions (Phase 1).
	Loader *agent.AgentLoader
	// TeamManager manages team state and membership.
	TeamManager *agent.TeamManager
	// LegacyRunner is the old callback — used as fallback when Runner is nil.
	LegacyRunner SubAgentRunner
	// ParentContext is the subagent context from the parent agent.
	ParentContext *agent.SubagentContext
	// ParentMessages are the parent's conversation messages (for fork).
	ParentMessages []*engine.Message
	// IsCoordinatorMode indicates the parent is in coordinator mode.
	IsCoordinatorMode bool
}

// AgentTool spawns a sub-agent to complete a task.
// Implements the 4-way decision tree from claude-code-main:
//  1. Teammate spawn → InProcess teammate (Phase 6)
//  2. Remote isolation → Remote agent stub (Phase 7)
//  3. Async lifecycle → Background agent with progress (Phase 3)
//  4. Sync run → Foreground agent (Phase 2)
type AgentTool struct {
	tool.BaseTool
	runSubAgent SubAgentRunner // legacy fallback
	cfg         AgentToolConfig
}

// New creates an AgentTool with legacy runner (backward compatible).
func New(runner SubAgentRunner) *AgentTool {
	return &AgentTool{
		runSubAgent: runner,
		cfg:         AgentToolConfig{LegacyRunner: runner},
	}
}

// NewWithConfig creates an AgentTool with the full multi-agent config.
func NewWithConfig(cfg AgentToolConfig) *AgentTool {
	return &AgentTool{
		runSubAgent: cfg.LegacyRunner,
		cfg:         cfg,
	}
}

func (t *AgentTool) Name() string                      { return "Task" }
func (t *AgentTool) UserFacingName() string            { return "task" }
func (t *AgentTool) Description() string               { return "Spawn a sub-agent to complete a task autonomously." }
func (t *AgentTool) IsReadOnly(_ json.RawMessage) bool { return false }
func (t *AgentTool) GetActivityDescription(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return "Running sub-agent"
	}
	if in.Description != "" {
		return "Agent: " + in.Description
	}
	task := in.Task
	if len(task) > 50 {
		task = task[:50] + "…"
	}
	return "Agent: " + task
}
func (t *AgentTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *AgentTool) MaxResultSizeChars() int                  { return 50_000 }
func (t *AgentTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *AgentTool) IsTransparentWrapper() bool               { return true }

func (t *AgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"task":{"type":"string","description":"Full description of the task for the sub-agent. Be thorough — the agent starts fresh."},
			"description":{"type":"string","description":"Short 3-5 word summary of the task (shown in UI)."},
			"allowed_tools":{"type":"array","items":{"type":"string"},"description":"Optional list of tool names the sub-agent may use."},
			"max_turns":{"type":"integer","description":"Maximum turns for the sub-agent (default 50)."},
			"system_prompt":{"type":"string","description":"Optional custom system prompt for the sub-agent."},
			"subagent_type":{"type":"string","description":"Specialized agent type (e.g. general, explore, plan, verify, or custom)."},
			"run_in_background":{"type":"boolean","description":"If true, run the agent in the background and return immediately."},
			"model":{"type":"string","description":"Optional model override (e.g. sonnet, opus, haiku)."},
			"isolation":{"type":"string","enum":["worktree","remote",""],"description":"Isolation mode. worktree: git worktree. remote: container."},
			"teammate":{"type":"boolean","description":"If true, spawn as an in-process teammate with mailbox communication."},
			"fork":{"type":"boolean","description":"If true, fork current conversation context for prompt cache sharing."},
			"name":{"type":"string","description":"Name for the teammate (required with team_name for swarm spawn)."},
			"team_name":{"type":"string","description":"Team to spawn the teammate into."},
			"mode":{"type":"string","enum":["plan",""],"description":"Agent mode. plan: forces plan mode."},
			"cwd":{"type":"string","description":"Working directory override for the agent."}
		},
		"required":["task"]
	}`)
}

func (t *AgentTool) Prompt(uctx *tool.UseContext) string {
	// Use the dynamic prompt builder if a loader is available.
	if t.cfg.Loader != nil {
		agents := t.cfg.Loader.MergeAll()
		filtered := t.cfg.Loader.FilterByMcpAvailability(agents)
		return agent.BuildAgentToolPrompt(filtered, false)
	}

	// Legacy static prompt.
	return `Launch a new agent to handle complex, multi-step tasks autonomously.

The Task tool launches specialized agents (subprocesses) that autonomously handle complex tasks.

When NOT to use the Task tool:
- If you want to read a specific file path, use the Read tool or Glob tool instead, to find the match more quickly
- If you are searching for a specific class definition like "class Foo", use Glob/Grep instead, to find the match more quickly
- If you are searching for code within a specific file or set of 2-3 files, use the Read tool instead

Usage notes:
- Always include a short description summarizing what the agent will do
- Launch multiple agents concurrently whenever possible, to maximize performance; to do that, use a single message with multiple tool uses
- When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary.
- Each Agent invocation starts fresh — provide a complete task description.
- The agent's outputs should generally be trusted
- Clearly tell the agent whether you expect it to write code or just to do research (search, file reads, web fetches, etc.)

Writing the prompt:
- Brief the agent like a smart colleague who just walked into the room — it hasn't seen this conversation, doesn't know what you've tried, doesn't understand why this task matters.
- Explain what you're trying to accomplish and why.
- Describe what you've already learned or ruled out.
- Give enough context about the surrounding problem that the agent can make judgment calls rather than just following a narrow instruction.`
}

func (t *AgentTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Task == "" {
		return fmt.Errorf("task must not be empty")
	}
	if in.MaxTurns < 0 {
		return fmt.Errorf("max_turns must be non-negative")
	}
	if in.MaxTurns > 200 {
		return fmt.Errorf("max_turns exceeds maximum of 200")
	}
	return nil
}

func (t *AgentTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Task == "" {
		return fmt.Errorf("task must not be empty")
	}
	return nil
}

func (t *AgentTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Resolve model: in coordinator mode, ignore model param (coordinator picks).
	if t.cfg.IsCoordinatorMode {
		in.Model = ""
	}

	// Resolve team name from input or parent context.
	teamName := t.resolveTeamName(in)

	// Validation: teammates cannot spawn other teammates.
	if t.cfg.ParentContext != nil && t.cfg.ParentContext.TeamName != "" && teamName != "" && in.Name != "" {
		ch := make(chan *engine.ContentBlock, 1)
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Teammates cannot spawn other teammates \u2014 the team roster is flat. To spawn a subagent instead, omit the `name` parameter.", IsError: true}
		close(ch)
		return ch, nil
	}

	// Apply subagent type defaults if no explicit allowed_tools.
	if len(in.AllowedTools) == 0 && in.SubagentType != "" {
		if tools, ok := subagentTypes[in.SubagentType]; ok && tools != nil {
			in.AllowedTools = tools
		}
	}

	// Apply cwd override.
	if in.Cwd != "" {
		uctx.WorkDir = in.Cwd
	}

	ch := make(chan *engine.ContentBlock, 4)
	go func() {
		defer close(ch)

		agentID := uuid.New().String()

		// ── Pre-decision: teammate spawn check (team_name + name) ───────
		// Aligned with claude-code-main: if teamName && name, spawn teammate.
		if teamName != "" && in.Name != "" {
			t.handleTeammateSpawn(ctx, agentID, in, teamName, uctx, ch)
			return
		}

		// ── 4-Way Decision Tree ──────────────────────────────────
		// Aligned with claude-code-main's agentTool.ts decision paths.
		path := t.resolveDecisionPath(in)

		slog.Info("agentool: decision",
			slog.String("path", string(path)),
			slog.String("agent_id", agentID),
			slog.String("type", in.SubagentType),
			slog.String("team", teamName),
		)

		switch path {
		case decisionPathFork:
			t.handleForkPath(ctx, agentID, in, uctx, ch)
		case decisionPathTeammate:
			t.handleTeammatePath(ctx, agentID, in, uctx, ch)
		case decisionPathRemote:
			t.handleRemotePath(ctx, agentID, in, uctx, ch)
		case decisionPathAsync:
			t.handleAsyncPath(ctx, agentID, in, uctx, ch)
		default: // decisionPathSync
			t.handleSyncPath(ctx, agentID, in, uctx, ch)
		}
	}()
	return ch, nil
}

// ── Decision Path Types ──────────────────────────────────────────────────────

type decisionPath string

const (
	decisionPathSync     decisionPath = "sync"
	decisionPathAsync    decisionPath = "async"
	decisionPathTeammate decisionPath = "teammate"
	decisionPathRemote   decisionPath = "remote"
	decisionPathFork     decisionPath = "fork"
)

// resolveDecisionPath determines which execution path to take.
// Priority: fork > teammate > remote > async > sync.
// Aligned with claude-code-main's AgentTool.call decision tree.
func (t *AgentTool) resolveDecisionPath(in Input) decisionPath {
	// Resolve effective isolation and background from agent definition.
	effIsolation := agent.IsolationMode(in.Isolation)
	effBackground := in.RunInBackground

	if t.cfg.Loader != nil && in.SubagentType != "" {
		if def, ok := t.cfg.Loader.FindByType(in.SubagentType); ok {
			if effIsolation == "" {
				effIsolation = def.Isolation
			}
			if !effBackground {
				effBackground = def.Background
			}
		}
	}

	// Path 0: Fork — auto-detect if should fork.
	if in.Fork || agent.ShouldFork(t.cfg.ParentContext, t.cfg.ParentMessages, effIsolation, effBackground) {
		return decisionPathFork
	}

	// Path 1: Teammate spawn (explicit flag).
	if in.Teammate {
		return decisionPathTeammate
	}

	// Path 2: Remote isolation.
	if effIsolation == agent.IsolationRemote {
		return decisionPathRemote
	}

	// Path 3: Async lifecycle.
	if effBackground {
		return decisionPathAsync
	}

	// Path 4: Sync (default).
	return decisionPathSync
}

// resolveTeamName resolves the effective team name from input or parent context.
// Aligned with claude-code-main's resolveTeamName.
func (t *AgentTool) resolveTeamName(in Input) string {
	if in.TeamName != "" {
		return in.TeamName
	}
	if t.cfg.ParentContext != nil && t.cfg.ParentContext.TeamName != "" {
		return t.cfg.ParentContext.TeamName
	}
	return ""
}

// ── Path Handlers ────────────────────────────────────────────────────────────

// handleSyncPath executes the agent synchronously (foreground).
func (t *AgentTool) handleSyncPath(ctx context.Context, agentID string, in Input, uctx *tool.UseContext, ch chan<- *engine.ContentBlock) {
	// Try new runner first.
	if t.cfg.Runner != nil {
		params := t.buildRunParams(agentID, in, uctx)
		result := t.cfg.Runner.RunAgent(ctx, params)
		if result.Error != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: result.Error.Error(), IsError: true}
			return
		}
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: agent.FormatAgentResult(result, 50000)}
		return
	}

	// Legacy fallback.
	if t.runSubAgent == nil {
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Sub-agent runner not configured.", IsError: true}
		return
	}

	// Emit progress indicator.
	if uctx.SetToolJSX != nil {
		uctx.SetToolJSX(uctx.ToolUseID, map[string]string{"status": "running", "agentId": agentID})
	}

	result, err := t.runSubAgent(ctx, agentID, in.Task, in, uctx)
	if err != nil {
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
		return
	}
	ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: result}
}

// handleAsyncPath launches the agent in the background and returns immediately.
func (t *AgentTool) handleAsyncPath(ctx context.Context, agentID string, in Input, uctx *tool.UseContext, ch chan<- *engine.ContentBlock) {
	// Try async manager first.
	if t.cfg.AsyncManager != nil && t.cfg.Runner != nil {
		params := t.buildRunParams(agentID, in, uctx)
		params.Background = true
		launchedID, err := t.cfg.AsyncManager.Launch(ctx, params)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Started background agent %s. Task: %s", launchedID, in.Description),
		}
		return
	}

	// Legacy fallback.
	if t.runSubAgent != nil {
		go func() {
			_, _ = t.runSubAgent(ctx, agentID, in.Task, in, uctx)
		}()
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Started background agent %s. Task: %s", agentID, in.Description),
		}
		return
	}

	ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Sub-agent runner not configured.", IsError: true}
}

// handleForkPath launches a fork subagent with prompt cache sharing.
func (t *AgentTool) handleForkPath(ctx context.Context, agentID string, in Input, uctx *tool.UseContext, ch chan<- *engine.ContentBlock) {
	if t.cfg.Runner == nil {
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Agent runner not configured for fork.", IsError: true}
		return
	}

	params := agent.ForkAgentParams(
		in.Task,
		t.cfg.ParentMessages,
		"", // parent system prompt
		uctx.WorkDir,
		t.cfg.ParentContext,
	)
	params.Description = in.Description

	// Fork always runs async.
	if t.cfg.AsyncManager != nil {
		launchedID, err := t.cfg.AsyncManager.Launch(ctx, params)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Forked agent %s started in worktree. Task: %s", launchedID, in.Description),
		}
		return
	}

	// Fallback: run fork synchronously.
	result := t.cfg.Runner.RunAgent(ctx, params)
	ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: agent.SummarizeForkResult(result)}
}

// handleTeammateSpawn spawns a named teammate into a team.
// Triggered when team_name + name are both provided.
func (t *AgentTool) handleTeammateSpawn(ctx context.Context, agentID string, in Input, teamName string, uctx *tool.UseContext, ch chan<- *engine.ContentBlock) {
	slog.Info("agentool: spawning teammate",
		slog.String("name", in.Name),
		slog.String("team", teamName),
		slog.String("agent_id", agentID),
	)

	// Resolve agent definition for the teammate.
	var agentDef *agent.AgentDefinition
	if t.cfg.Loader != nil && in.SubagentType != "" {
		if def, ok := t.cfg.Loader.FindByType(in.SubagentType); ok {
			agentDef = def
		}
	}

	params := agent.RunAgentParams{
		AgentDef:            agentDef,
		Task:                in.Task,
		WorkDir:             uctx.WorkDir,
		Model:               in.Model,
		MaxTurns:            in.MaxTurns,
		AllowedTools:        in.AllowedTools,
		TeamName:            teamName,
		Background:          true,
		IsInProcessTeammate: true,
		Description:         in.Description,
		ExistingAgentID:     agentID,
		ParentContext:       t.cfg.ParentContext,
	}

	if in.Mode == "plan" {
		params.PermissionMode = "plan"
	}

	// Launch via async manager if available.
	if t.cfg.AsyncManager != nil {
		launchedID, err := t.cfg.AsyncManager.Launch(ctx, params)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Teammate '%s' spawned in team '%s' (id: %s). Task: %s", in.Name, teamName, launchedID, in.Description),
		}
		return
	}

	// Fallback: run synchronously.
	if t.cfg.Runner != nil {
		result := t.cfg.Runner.RunAgent(ctx, params)
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: agent.FormatAgentResult(result, 50000)}
		return
	}

	ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Agent runner not configured for teammate spawn.", IsError: true}
}

// handleTeammatePath spawns an in-process teammate agent (explicit flag, no team).
func (t *AgentTool) handleTeammatePath(ctx context.Context, agentID string, in Input, uctx *tool.UseContext, ch chan<- *engine.ContentBlock) {
	// Delegate to async path with IsInProcessTeammate flag.
	slog.Info("agentool: teammate path delegating to async", slog.String("agent_id", agentID))
	in.RunInBackground = true
	t.handleAsyncPath(ctx, agentID, in, uctx, ch)
}

// handleRemotePath spawns a remote (container-isolated) agent.
func (t *AgentTool) handleRemotePath(ctx context.Context, agentID string, in Input, uctx *tool.UseContext, ch chan<- *engine.ContentBlock) {
	// Phase 7 will implement RemoteBackend.
	// For now, fall back to async path with worktree isolation.
	slog.Info("agentool: remote path delegating to async with worktree", slog.String("agent_id", agentID))
	in.Isolation = "worktree" // downgrade to worktree
	in.RunInBackground = true
	t.handleAsyncPath(ctx, agentID, in, uctx, ch)
}

// buildRunParams converts Input + UseContext into RunAgentParams.
func (t *AgentTool) buildRunParams(agentID string, in Input, uctx *tool.UseContext) agent.RunAgentParams {
	// Resolve agent definition from loader.
	var agentDef *agent.AgentDefinition
	if t.cfg.Loader != nil && in.SubagentType != "" {
		if def, ok := t.cfg.Loader.FindByType(in.SubagentType); ok {
			agentDef = def
		}
	}

	params := agent.RunAgentParams{
		AgentDef:            agentDef,
		Task:                in.Task,
		WorkDir:             uctx.WorkDir,
		Model:               in.Model,
		MaxTurns:            in.MaxTurns,
		AllowedTools:        in.AllowedTools,
		SystemPrompt:        in.SystemPrompt,
		Background:          in.RunInBackground,
		IsolationMode:       agent.IsolationMode(in.Isolation),
		PermissionMode:      uctx.PermissionMode,
		ExistingAgentID:     agentID,
		IsFork:              in.Fork,
		IsInProcessTeammate: in.Teammate,
		Description:         in.Description,
		ParentContext:       t.cfg.ParentContext,
		TeamName:            t.resolveTeamName(in),
	}

	if in.Mode == "plan" {
		params.PermissionMode = "plan"
	}

	return params
}
