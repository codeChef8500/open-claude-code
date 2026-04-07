package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/analytics"
	"github.com/wall-ai/agent-engine/internal/buddy"
	"github.com/wall-ai/agent-engine/internal/command"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/prompt"
)

// Runner orchestrates the interactive message loop, connecting
// user input → engine → output.
type Runner struct {
	result *BootstrapResult

	// Callbacks for UI integration.
	OnTextDelta    func(text string)
	OnToolStart    func(id, name, input string)
	OnToolDone     func(id, name, output string, isError bool)
	OnToolProgress func(id, name, content string)
	OnDone         func()
	OnError        func(err error)
	OnSystem       func(text string)
	OnClearHistory func()
	OnCompact      func()

	// Companion callbacks — called by handleBuddySignal to sync TUI state.
	OnCompanionLoad     func(comp *buddy.Companion)
	OnCompanionPet      func(tsMs int64)
	OnCompanionMute     func(muted bool)
	OnCompanionReaction func(text string)

	// Observer fires companion reactions based on engine events.
	observer *buddy.Observer
}

// NewRunner creates a runner from bootstrap results.
func NewRunner(result *BootstrapResult) *Runner {
	return &Runner{
		result:              result,
		OnTextDelta:         func(string) {},
		OnToolStart:         func(string, string, string) {},
		OnToolDone:          func(string, string, string, bool) {},
		OnToolProgress:      func(string, string, string) {},
		OnDone:              func() {},
		OnError:             func(error) {},
		OnSystem:            func(string) {},
		OnClearHistory:      func() {},
		OnCompact:           func() {},
		OnCompanionLoad:     func(*buddy.Companion) {},
		OnCompanionPet:      func(int64) {},
		OnCompanionMute:     func(bool) {},
		OnCompanionReaction: func(string) {},
	}
}

// SetObserver attaches a buddy observer that fires companion reactions.
func (r *Runner) SetObserver(obs *buddy.Observer) {
	r.observer = obs
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
		slog.Debug("HandleInput: detected slash command", slog.String("input", input))
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
		slog.Debug("handleCommand: executor error", slog.String("cmd", cmdName), slog.String("error", err.Error()))
		r.OnError(fmt.Errorf("command error: %w", err))
		r.OnDone()
		return true
	}

	slog.Debug("handleCommand: executor success", slog.String("cmd", cmdName), slog.String("output_prefix", truncateForLog(output, 80)))

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

	case output == "__clear_history__" || strings.HasPrefix(output, "__clear_history__\n"):
		r.OnClearHistory()
		// Show any extra info after the signal (e.g. "Messages cleared\n...").
		if rest := strings.TrimPrefix(output, "__clear_history__"); rest != "" {
			r.OnSystem(strings.TrimPrefix(rest, "\n"))
		}
		r.OnDone()
		return true

	case output == "__compact__" || strings.HasPrefix(output, "__compact__\n"):
		r.OnCompact()
		if rest := strings.TrimPrefix(output, "__compact__"); rest != "" {
			r.OnSystem(strings.TrimPrefix(rest, "\n"))
		}
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
		rest := strings.TrimPrefix(output, "__interactive__:")
		// If the executor embedded fallback text after the component name
		// (separated by "\n"), display that instead of the generic stub.
		if idx := strings.Index(rest, "\n"); idx >= 0 {
			fallback := rest[idx+1:]
			r.OnSystem(fallback)
		} else {
			r.OnSystem(formatInteractiveResult(rest))
		}
		r.OnDone()
		return true

	case command.IsBuddySignal(output):
		r.handleBuddySignal(command.ParseBuddySignal(output))
		return true

	default:
		if output != "" {
			r.OnSystem(output)
		}
		r.OnDone()
		return true
	}
}

// truncateForLog returns at most maxLen characters of s for logging.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// BuddyConfigDir returns the config directory for buddy storage.
func BuddyConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// handleBuddySignal processes buddy command signals.
// It bridges between the signal-based /buddy command and the buddy package,
// performing hatch, show card, pet, mute/unmute, and stats actions.
func (r *Runner) handleBuddySignal(action string) {
	configDir := BuddyConfigDir()
	userID := buddy.GetOrCreateUserID(configDir)

	switch action {
	case "show":
		// Load or hatch companion
		comp := buddy.LoadCompanion(userID, configDir)
		if comp == nil {
			// Hatch a new companion (without LLM for now — fast path)
			comp = buddy.HatchWithoutLLM(userID)
			if comp != nil {
				if err := buddy.SaveCompanion(comp, configDir); err != nil {
					slog.Warn("buddy: failed to save companion", slog.String("error", err.Error()))
				}
			}
		}
		if comp == nil {
			r.OnSystem("Could not hatch companion. Try again later.")
			r.OnDone()
			return
		}
		// Update TUI companion widget
		r.OnCompanionLoad(comp)
		// Create observer if not yet set (first hatch during session)
		if r.observer == nil {
			r.observer = buddy.NewObserver(comp, r.OnCompanionReaction)
		} else {
			r.observer.SetCompanion(comp)
		}
		// Render card
		spriteLines := buddy.RenderSprite(comp.CompanionBones, 0)
		stars := buddy.RarityStars[comp.Rarity]
		card := command.BuddyCardText(
			comp.Name,
			string(comp.Species),
			string(comp.Rarity),
			stars,
			comp.Personality,
			comp.Shiny,
			fmt.Sprintf("%d", comp.HatchedAt),
			spriteLines,
		)
		r.OnSystem(card)
		r.OnDone()

	case "pet":
		r.OnCompanionPet(time.Now().UnixMilli())
		r.OnSystem("*You pet your companion* ♥")
		r.OnDone()

	case "mute":
		if err := buddy.SetCompanionMuted(true, configDir); err != nil {
			r.OnError(fmt.Errorf("buddy mute: %w", err))
		} else {
			r.OnCompanionMute(true)
			r.OnSystem("Companion muted. Use /buddy unmute to bring them back.")
		}
		r.OnDone()

	case "unmute":
		if err := buddy.SetCompanionMuted(false, configDir); err != nil {
			r.OnError(fmt.Errorf("buddy unmute: %w", err))
		} else {
			r.OnCompanionMute(false)
			// Reload companion into TUI after unmute
			comp := buddy.LoadCompanion(userID, configDir)
			if comp != nil {
				r.OnCompanionLoad(comp)
			}
			r.OnSystem("Companion unmuted!")
		}
		r.OnDone()

	case "stats":
		comp := buddy.LoadCompanion(userID, configDir)
		if comp == nil {
			r.OnSystem("No companion yet. Use /buddy to hatch one!")
			r.OnDone()
			return
		}
		stars := buddy.RarityStars[comp.Rarity]
		statMap := make(map[string]int, len(comp.Stats))
		statOrder := make([]string, 0, len(buddy.AllStatNames))
		for _, sn := range buddy.AllStatNames {
			statMap[string(sn)] = comp.Stats[sn]
			statOrder = append(statOrder, string(sn))
		}
		card := command.BuddyStatsText(
			comp.Name,
			string(comp.Rarity),
			stars,
			string(comp.Eye),
			string(comp.Hat),
			comp.Shiny,
			statMap,
			statOrder,
		)
		r.OnSystem(card)
		r.OnDone()

	default:
		r.OnSystem(fmt.Sprintf("Unknown buddy action: %s", action))
		r.OnDone()
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

	// Fire observer: user message + turn start
	if r.observer != nil {
		r.observer.OnEvent(buddy.EngineEvent{Kind: buddy.EventUserMessage, Detail: text})
		r.observer.OnEvent(buddy.EngineEvent{Kind: buddy.EventTurnStart})
	}

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
				if data, err := json.Marshal(ev.ToolInput); err == nil {
					inputStr = string(data)
				}
			}
			r.OnToolStart(ev.ToolID, ev.ToolName, inputStr)
			r.result.SessionTracker.RecordToolCall(ev.ToolName, false)
			if r.observer != nil {
				r.observer.OnEvent(buddy.EngineEvent{Kind: buddy.EventToolStart, ToolName: ev.ToolName})
			}

		case engine.EventToolProgress:
			content := ""
			if ev.Progress != nil {
				content = ev.Progress.Content
			}
			r.OnToolProgress(ev.ToolID, ev.ToolName, content)

		case engine.EventToolResult:
			r.OnToolDone(ev.ToolID, ev.ToolName, ev.Text, ev.IsError)
			if ev.IsError {
				r.result.SessionTracker.RecordToolCall("", true)
			}
			if r.observer != nil {
				r.observer.OnEvent(buddy.EngineEvent{Kind: buddy.EventToolEnd, ToolName: ev.ToolID})
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
			if r.observer != nil {
				r.observer.OnEvent(buddy.EngineEvent{Kind: buddy.EventError, Detail: ev.Error})
			}

		case engine.EventDone:
			r.result.SessionTracker.RecordAssistantMessage()
			r.result.SessionTracker.RecordTurn()
			if r.observer != nil {
				r.observer.OnEvent(buddy.EngineEvent{Kind: buddy.EventTurnEnd})
			}
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
