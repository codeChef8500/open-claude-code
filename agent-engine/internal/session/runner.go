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
		return false
	}

	ectx := &command.ExecContext{
		WorkDir:   r.result.Engine.WorkDir(),
		SessionID: r.result.Engine.SessionID(),
	}

	output, err := r.result.CmdExecutor.Execute(ctx, input, ectx)
	if err != nil {
		r.OnError(fmt.Errorf("command error: %w", err))
		return true
	}
	if output != "" {
		r.OnSystem(output)
	}

	r.result.SessionTracker.RecordCommand()
	analytics.LogEvent("command_executed", analytics.EventMetadata{
		"command": cmdName,
	})

	return true
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
