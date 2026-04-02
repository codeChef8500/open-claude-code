package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultHookTimeout = 60 * time.Second

// Executor runs hook scripts for lifecycle events.
// Aligned with claude-code-main's hook execution logic in hooks.ts.
type Executor struct {
	hooks     HooksSettings
	cwd       string
	sessionID string
}

// NewExecutor creates a hook executor from the given settings.
func NewExecutor(settings HooksSettings, cwd, sessionID string) *Executor {
	return &Executor{
		hooks:     settings,
		cwd:       cwd,
		sessionID: sessionID,
	}
}

// HasHooksFor reports whether any hooks are registered for the given event.
func (e *Executor) HasHooksFor(event HookEvent) bool {
	hooks, ok := e.hooks[event]
	return ok && len(hooks) > 0
}

// RunSync executes all synchronous hooks for the given event and returns
// the merged result. If any hook blocks or stops, execution halts.
func (e *Executor) RunSync(ctx context.Context, event HookEvent, input *HookInput) SyncHookResponse {
	hooks, ok := e.hooks[event]
	if !ok || len(hooks) == 0 {
		return SyncHookResponse{}
	}

	// Fill in common fields.
	input.Event = event
	input.SessionID = e.sessionID
	input.CWD = e.cwd
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now()
	}

	var merged SyncHookResponse
	for _, cfg := range hooks {
		if cfg.Async {
			continue // async hooks are fire-and-forget
		}

		resp := e.runOne(ctx, cfg, input)
		if resp.Error != nil {
			merged.Error = resp.Error
			return merged
		}

		// Merge: later hooks can override earlier ones.
		if resp.Decision != "" {
			merged.Decision = resp.Decision
		}
		if resp.ShouldStop {
			merged.ShouldStop = true
			merged.StopReason = resp.StopReason
		}
		if resp.UpdatedInput != nil {
			merged.UpdatedInput = resp.UpdatedInput
		}
		if resp.AdditionalContext != "" {
			merged.AdditionalContext = resp.AdditionalContext
		}
		if resp.OutputOverride != nil {
			merged.OutputOverride = resp.OutputOverride
		}
		if resp.Passed != nil {
			merged.Passed = resp.Passed
		}
		if resp.FailureReason != "" {
			merged.FailureReason = resp.FailureReason
		}
		if resp.NewCustomInstructions != "" {
			merged.NewCustomInstructions = resp.NewCustomInstructions
		}

		// If this hook blocked, stop executing further hooks.
		if resp.Decision == "block" {
			break
		}
	}

	return merged
}

// RunAsync fires all async hooks for the given event without blocking.
func (e *Executor) RunAsync(event HookEvent, input *HookInput) {
	hooks, ok := e.hooks[event]
	if !ok || len(hooks) == 0 {
		return
	}

	input.Event = event
	input.SessionID = e.sessionID
	input.CWD = e.cwd
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now()
	}

	for _, cfg := range hooks {
		if !cfg.Async {
			continue
		}
		go func(c HookConfig) {
			timeout := resolveTimeout(c)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			_ = e.runOne(ctx, c, input)
		}(cfg)
	}
}

// RunAll runs sync hooks and fires async hooks for the given event.
func (e *Executor) RunAll(ctx context.Context, event HookEvent, input *HookInput) SyncHookResponse {
	e.RunAsync(event, input)
	return e.RunSync(ctx, event, input)
}

// runOne executes a single hook script and parses its output.
func (e *Executor) runOne(ctx context.Context, cfg HookConfig, input *HookInput) SyncHookResponse {
	timeout := resolveTimeout(cfg)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Serialize input to JSON for stdin.
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return SyncHookResponse{Error: fmt.Errorf("marshal hook input: %w", err)}
	}

	// Build command.
	args := append([]string{}, cfg.Args...)
	cmd := exec.CommandContext(ctx, cfg.Command, args...)
	cmd.Dir = e.cwd
	cmd.Stdin = bytes.NewReader(inputJSON)

	// Set environment.
	if len(cfg.Env) > 0 {
		env := cmd.Environ()
		for k, v := range cfg.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Non-zero exit is treated as a block decision for PreToolUse,
		// or a failure for other events.
		if ctx.Err() == context.DeadlineExceeded {
			return SyncHookResponse{Error: fmt.Errorf("hook %q timed out after %v", cfg.Command, timeout)}
		}
		return SyncHookResponse{
			Decision: "block",
			Error:    fmt.Errorf("hook %q failed: %w (stderr: %s)", cfg.Command, err, strings.TrimSpace(stderr.String())),
		}
	}

	// Parse JSON output.
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return SyncHookResponse{} // no output = no opinion
	}

	var raw HookJSONOutput
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return SyncHookResponse{Error: fmt.Errorf("parse hook output: %w (raw: %s)", err, output)}
	}

	return parseHookOutput(raw)
}

// parseHookOutput converts raw JSON output to a SyncHookResponse.
func parseHookOutput(raw HookJSONOutput) SyncHookResponse {
	resp := SyncHookResponse{
		Decision:              raw.Decision,
		ShouldStop:            raw.ShouldStop,
		StopReason:            raw.StopReason,
		AdditionalContext:     raw.AdditionalContext,
		OutputOverride:        raw.OutputOverride,
		Passed:                raw.Passed,
		FailureReason:         raw.FailureReason,
		NewCustomInstructions: raw.NewCustomInstructions,
	}

	if raw.UpdatedInput != nil {
		resp.UpdatedInput = raw.UpdatedInput
	}

	// `continue: false` is equivalent to decision=block.
	if raw.Continue != nil && !*raw.Continue && resp.Decision == "" {
		resp.Decision = "block"
		if raw.Reason != "" {
			resp.FailureReason = raw.Reason
		}
	}

	return resp
}

func resolveTimeout(cfg HookConfig) time.Duration {
	if cfg.TimeoutSeconds > 0 {
		return time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	return defaultHookTimeout
}
