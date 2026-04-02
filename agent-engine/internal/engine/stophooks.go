package engine

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// StopHookTrigger identifies the event that fired a stop hook.
type StopHookTrigger string

const (
	StopHookOnStop            StopHookTrigger = "Stop"
	StopHookOnSubagentStop    StopHookTrigger = "SubagentStop"
	StopHookOnPreToolUse      StopHookTrigger = "PreToolUse"
	StopHookOnPostToolUse     StopHookTrigger = "PostToolUse"
	StopHookOnNotification    StopHookTrigger = "Notification"
)

// StopHookConfig is a single user-configured stop hook entry.
type StopHookConfig struct {
	// Trigger is the event that fires this hook.
	Trigger StopHookTrigger
	// Command is the shell command to execute.
	Command string
	// TimeoutMs is the execution timeout in milliseconds (default 60_000).
	TimeoutMs int
	// RunInBackground, if true, does not block the agent loop.
	RunInBackground bool
}

// StopHookResult captures the outcome of a hook execution.
type StopHookResult struct {
	Trigger    StopHookTrigger
	Command    string
	ExitCode   int
	Stdout     string
	Stderr     string
	Duration   time.Duration
	// Continue, if false, means the hook is requesting the agent loop to halt.
	Continue bool
}

// StopHookExecutor runs a list of stop hooks for a given trigger event.
type StopHookExecutor struct {
	hooks []StopHookConfig
}

// NewStopHookExecutor creates an executor from the provided hook list.
func NewStopHookExecutor(hooks []StopHookConfig) *StopHookExecutor {
	return &StopHookExecutor{hooks: hooks}
}

// Run fires all hooks that match trigger.  It blocks until all non-background
// hooks have completed.  Returns the first result that requests a stop
// (Continue=false), or nil if all hooks pass.
func (e *StopHookExecutor) Run(ctx context.Context, trigger StopHookTrigger, env []string) *StopHookResult {
	for _, h := range e.hooks {
		if h.Trigger != trigger {
			continue
		}
		if h.RunInBackground {
			go e.execute(ctx, h, env) //nolint:errcheck
			continue
		}
		result := e.execute(ctx, h, env)
		if result != nil && !result.Continue {
			return result
		}
	}
	return nil
}

func (e *StopHookExecutor) execute(ctx context.Context, h StopHookConfig, env []string) *StopHookResult {
	timeout := time.Duration(h.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	start := time.Now()

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", h.Command)
	if len(env) > 0 {
		cmd.Env = env
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			slog.Warn("stop hook exec error",
				slog.String("trigger", string(h.Trigger)),
				slog.String("command", h.Command),
				slog.Any("err", err))
		}
	}

	result := &StopHookResult{
		Trigger:  h.Trigger,
		Command:  h.Command,
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
		Continue: exitCode == 0,
	}

	slog.Debug("stop hook executed",
		slog.String("trigger", string(h.Trigger)),
		slog.Int("exit_code", exitCode),
		slog.Duration("duration", result.Duration))

	return result
}
