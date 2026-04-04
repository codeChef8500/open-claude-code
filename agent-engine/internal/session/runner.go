package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wall-ai/agent-engine/internal/analytics"
	"github.com/wall-ai/agent-engine/internal/command"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/prompt"
)

// Runner orchestrates the interactive message loop, connecting
// user input → engine → output.
type Runner struct {
	result *BootstrapResult

	// Callbacks for UI integration.
	OnTextDelta func(text string)
	OnToolStart func(id, name, input string)
	OnToolDone  func(id, output string, isError bool)
	OnDone      func()
	OnError     func(err error)
	OnSystem    func(text string)
}

// NewRunner creates a runner from bootstrap results.
func NewRunner(result *BootstrapResult) *Runner {
	return &Runner{
		result:      result,
		OnTextDelta: func(string) {},
		OnToolStart: func(string, string, string) {},
		OnToolDone:  func(string, string, bool) {},
		OnDone:      func() {},
		OnError:     func(error) {},
		OnSystem:    func(string) {},
	}
}

// HandleInput processes a single user input (message or slash command).
// Returns true if the session should continue, false to exit.
func (r *Runner) HandleInput(ctx context.Context, input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return true
	}

	// Check for slash commands.
	if strings.HasPrefix(input, "/") {
		return r.handleCommand(ctx, input)
	}

	// Regular message → engine.
	r.handleMessage(ctx, input)
	return true
}

// handleCommand dispatches a slash command.
func (r *Runner) handleCommand(ctx context.Context, input string) bool {
	parts := strings.Fields(input)
	cmdName := strings.TrimPrefix(parts[0], "/")

	// Special exit commands.
	switch strings.ToLower(cmdName) {
	case "quit", "exit", "q":
		r.OnSystem("Goodbye!")
		r.OnDone()
		return false
	}

	ectx := r.buildExecContext()

	output, err := r.result.CmdExecutor.Execute(ctx, input, ectx)
	if err != nil {
		r.OnError(fmt.Errorf("command error: %w", err))
		r.OnDone()
		return true
	}

	r.result.SessionTracker.RecordCommand()
	analytics.LogEvent("command_executed", analytics.EventMetadata{
		"command": cmdName,
	})

	// Dispatch based on special return-value prefixes from the executor.
	return r.dispatchCommandResult(ctx, output)
}

// dispatchCommandResult handles the special return values from command execution.
func (r *Runner) dispatchCommandResult(ctx context.Context, output string) bool {
	switch {
	case output == "__quit__":
		r.OnSystem("Goodbye!")
		r.OnDone()
		return false

	case output == "__clear_history__":
		r.OnSystem("Conversation cleared.")
		r.OnDone()
		return true

	case output == "__compact__":
		r.OnSystem("Compacting conversation context…")
		r.OnDone()
		return true

	case strings.HasPrefix(output, "__prompt__:"):
		// Prompt command: forward content to the engine as a user message.
		promptContent := strings.TrimPrefix(output, "__prompt__:")
		r.handleMessage(ctx, promptContent)
		return true

	case strings.HasPrefix(output, "__fork_prompt__:"):
		// Forked prompt command: same as prompt (sub-agent not yet supported).
		promptContent := strings.TrimPrefix(output, "__fork_prompt__:")
		r.handleMessage(ctx, promptContent)
		return true

	case strings.HasPrefix(output, "__interactive__:"):
		// Interactive command: render a text-mode fallback.
		component := strings.TrimPrefix(output, "__interactive__:")
		r.OnSystem(formatInteractiveResult(component))
		r.OnDone()
		return true

	default:
		if output != "" {
			r.OnSystem(output)
		}
		r.OnDone()
		return true
	}
}

// buildExecContext creates a rich ExecContext from the current session state.
func (r *Runner) buildExecContext() *command.ExecContext {
	eng := r.result.Engine
	ectx := &command.ExecContext{
		WorkDir:   eng.WorkDir(),
		SessionID: eng.SessionID(),
	}
	// Pull config from the engine.
	if cfg := eng.Config(); cfg != nil {
		ectx.Model = cfg.Model
		ectx.AutoMode = cfg.AutoMode
		ectx.Verbose = cfg.Verbose
		ectx.PermissionMode = cfg.PermissionMode
	}
	// Pull dynamic state from the store.
	if s := eng.Store(); s != nil {
		if v := s.Get("cost_usd"); v != nil {
			if c, ok := v.(float64); ok {
				ectx.CostUSD = c
			}
		}
		if v := s.Get("turn_count"); v != nil {
			if tc, ok := v.(int); ok {
				ectx.TurnCount = tc
			}
		}
		if v := s.Get("total_tokens"); v != nil {
			if tt, ok := v.(int); ok {
				ectx.TotalTokens = tt
			}
		}
	}
	return ectx
}

// formatInteractiveResult returns human-readable text for interactive command
// components that cannot render a full TUI panel (text-mode fallback).
func formatInteractiveResult(component string) string {
	switch component {
	case "agents":
		return "Agent configurations — use /agents list or /agents add <name> to manage."
	case "tasks":
		return "Background tasks — no active tasks."
	case "memory":
		return "Memory files — edit CLAUDE.md in your project root or ~/.claude/CLAUDE.md for global memory."
	case "resume":
		return "Use /resume <session-id> to resume a previous conversation."
	case "session":
		return "Session info — use /status for current session details."
	case "permissions":
		return "Permission rules — use /permissions to view allowed and denied tools."
	case "plugin":
		return "Plugin management — use /plugin list to see installed plugins."
	case "skills":
		return "Skills — use /skills to list available skill commands."
	case "config":
		return "Configuration panel — use /config to view current settings."
	case "mcp":
		return "MCP server management — use /mcp list to see connected servers."
	case "plan":
		return "Plan mode toggled. Use /plan <message> to plan without executing."
	case "fast":
		return "Fast mode toggled (uses smaller, faster model for simple tasks)."
	case "effort":
		return "Effort level — use /effort [low|medium|high|max|auto] to set."
	case "theme":
		return "Theme — use /theme <name> to switch themes."
	case "branch":
		return "Branched current conversation."
	case "diff":
		return "Diff — showing uncommitted changes."
	case "review":
		return "Review — analyzing recent changes."
	case "login":
		return "Authentication — visit the URL shown to complete login."
	case "logout":
		return "Logged out."
	default:
		return fmt.Sprintf("/%s executed.", component)
	}
}

// handleMessage sends a user message through the engine.
func (r *Runner) handleMessage(ctx context.Context, text string) {
	r.result.SessionTracker.RecordUserMessage()

	// Process input (expand @file mentions, etc.).
	pi := prompt.ProcessUserInput(text, r.result.Engine.WorkDir(), nil)

	// Submit to engine.
	ch := r.result.Engine.SubmitMessage(ctx, engine.QueryParams{
		Text:   pi.Text,
		Source: engine.QuerySourceUser,
	})

	// Drain events.
	for ev := range ch {
		if ev == nil {
			continue
		}
		switch ev.Type {
		case engine.EventTextDelta:
			r.OnTextDelta(ev.Text)

		case engine.EventToolUse:
			inputStr := ""
			if ev.ToolInput != nil {
				inputStr = fmt.Sprintf("%v", ev.ToolInput)
			}
			r.OnToolStart(ev.ToolID, ev.ToolName, inputStr)
			r.result.SessionTracker.RecordToolCall(ev.ToolName, false)

		case engine.EventToolResult:
			r.OnToolDone(ev.ToolID, ev.Text, ev.IsError)
			if ev.IsError {
				r.result.SessionTracker.RecordToolCall("", true)
			}

		case engine.EventUsage:
			if ev.Usage != nil {
				r.result.CostTracker.RecordTurn(
					r.result.Engine.Store().GetString("model"),
					ev.Usage.InputTokens,
					ev.Usage.OutputTokens,
					ev.Usage.CacheCreationInputTokens,
					ev.Usage.CacheReadInputTokens,
				)
				r.result.SessionTracker.RecordAPIUsage(
					ev.Usage.InputTokens,
					ev.Usage.OutputTokens,
					ev.Usage.CacheReadInputTokens,
					ev.Usage.CacheCreationInputTokens,
					int64(ev.Usage.CostUSD*1_000_000),
					int64(ev.Usage.ServerDurationMs),
				)
			}

		case engine.EventError:
			r.OnError(fmt.Errorf("%s", ev.Error))

		case engine.EventDone:
			r.result.SessionTracker.RecordAssistantMessage()
			r.result.SessionTracker.RecordTurn()
			r.OnDone()

		case engine.EventSystemMessage:
			r.OnSystem(ev.Text)

		case engine.EventCompactBoundary:
			r.result.SessionTracker.RecordCompact()
			r.OnSystem("Context compacted.")

		default:
			slog.Debug("runner: unhandled event", slog.String("type", string(ev.Type)))
		}
	}
}
