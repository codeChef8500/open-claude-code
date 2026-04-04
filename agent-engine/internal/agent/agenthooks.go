package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// Agent hook execution aligned with claude-code-main's runAgent.ts hook lifecycle.
// Hooks are executed at defined lifecycle points: SubagentStart, SubagentEnd.

// HookEventType identifies which lifecycle event triggered the hook.
type HookEventType string

const (
	HookSubagentStart HookEventType = "SubagentStart"
	HookSubagentEnd   HookEventType = "SubagentEnd"
)

// HookContext provides data available to hook commands via environment variables.
type HookContext struct {
	AgentID    string
	AgentType  string
	ParentID   string
	TeamName   string
	WorkDir    string
	Task       string
	IsAsync    bool
	IsFork     bool
	Status     string // only set for SubagentEnd
	Output     string // only set for SubagentEnd (truncated)
	TurnCount  int    // only set for SubagentEnd
}

// ExecuteHooks runs all hook commands for the given event type on the agent definition.
// Hooks are run sequentially. A hook failure is logged but does not abort the agent.
func ExecuteHooks(ctx context.Context, eventType HookEventType, def *AgentDefinition, hookCtx *HookContext) {
	if def == nil || len(def.Hooks) == 0 {
		return
	}

	commands, ok := def.Hooks[string(eventType)]
	if !ok || len(commands) == 0 {
		return
	}

	for _, hc := range commands {
		if hc.Command == "" {
			continue
		}

		timeout := time.Duration(hc.Timeout) * time.Millisecond
		if timeout <= 0 {
			timeout = 30 * time.Second // default 30s timeout
		}

		hookCtxWithTimeout, cancel := context.WithTimeout(ctx, timeout)

		err := runHookCommand(hookCtxWithTimeout, hc.Command, hookCtx)
		cancel()

		if err != nil {
			slog.Warn("agent hook failed",
				slog.String("event", string(eventType)),
				slog.String("agent_id", hookCtx.AgentID),
				slog.String("command", hc.Command),
				slog.Any("err", err),
			)
		}
	}
}

// runHookCommand executes a single hook command with environment variables from HookContext.
func runHookCommand(ctx context.Context, command string, hookCtx *HookContext) error {
	// Use shell execution for the command string.
	var cmd *exec.Cmd
	cmd = exec.CommandContext(ctx, shellName(), shellFlag(), command)

	if hookCtx.WorkDir != "" {
		cmd.Dir = hookCtx.WorkDir
	}

	// Set environment variables from hook context.
	env := cmd.Environ()
	env = append(env,
		fmt.Sprintf("AGENT_ID=%s", hookCtx.AgentID),
		fmt.Sprintf("AGENT_TYPE=%s", hookCtx.AgentType),
		fmt.Sprintf("AGENT_TASK=%s", hookCtx.Task),
	)
	if hookCtx.ParentID != "" {
		env = append(env, fmt.Sprintf("AGENT_PARENT_ID=%s", hookCtx.ParentID))
	}
	if hookCtx.TeamName != "" {
		env = append(env, fmt.Sprintf("AGENT_TEAM_NAME=%s", hookCtx.TeamName))
	}
	if hookCtx.IsAsync {
		env = append(env, "AGENT_ASYNC=true")
	}
	if hookCtx.IsFork {
		env = append(env, "AGENT_IS_FORK=true")
	}
	if hookCtx.Status != "" {
		env = append(env, fmt.Sprintf("AGENT_STATUS=%s", hookCtx.Status))
	}
	if hookCtx.Output != "" {
		// Truncate output for env var to avoid exceeding OS limits.
		output := hookCtx.Output
		if len(output) > 4096 {
			output = output[:4096]
		}
		env = append(env, fmt.Sprintf("AGENT_OUTPUT=%s", output))
	}
	if hookCtx.TurnCount > 0 {
		env = append(env, fmt.Sprintf("AGENT_TURN_COUNT=%d", hookCtx.TurnCount))
	}
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hook command %q: %w (output: %s)", command, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// shellName returns the shell executable for the current platform.
func shellName() string {
	return "cmd"
}

// shellFlag returns the flag to pass a command string to the shell.
func shellFlag() string {
	return "/c"
}
