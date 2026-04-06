package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wall-ai/agent-engine/internal/buddy"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/session"
	"github.com/wall-ai/agent-engine/internal/tui"
	"github.com/wall-ai/agent-engine/internal/util"
)

// runInteractiveMode launches the full-screen Bubbletea TUI.
func runInteractiveMode(ctx context.Context, appCfg *util.AppConfig, wd string) error {
	result, err := session.Bootstrap(ctx, session.BootstrapConfig{
		AppConfig: appCfg,
		WorkDir:   wd,
	})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	defer session.Shutdown(result)

	runner := session.NewRunner(result)

	// We need a reference to the program for sending messages from goroutines.
	var program *tea.Program

	app, err := tui.NewApp(tui.AppConfig{
		Dark:           appCfg.DarkMode,
		Model:          appCfg.Model,
		PermissionMode: appCfg.PermissionMode,
		WorkDir:        wd,
		SubmitFn: func(text string) {
			// Run engine interaction in a goroutine so the TUI stays responsive.
			go func() {
				// BUG-1 fix: recover from panics so the TUI doesn't crash.
				defer func() {
					if r := recover(); r != nil {
						slog.Error("panic in command handler", slog.Any("error", r))
						if program != nil {
							program.Send(tui.StreamErrorMsg{
								Err: fmt.Errorf("internal error: %v", r),
							})
							program.Send(tui.StreamDoneMsg{})
						}
					}
				}()
				handleInteractiveInput(ctx, runner, program, text)
			}()
		},
	})
	if err != nil {
		return fmt.Errorf("create TUI: %w", err)
	}

	program = tea.NewProgram(app, tea.WithAltScreen())

	// BUG-7 fix: wire callbacks once to avoid per-submission data race.
	wireRunnerCallbacks(runner, program)

	// P3+P5: Auto-load companion on startup; auto-hatch if none exists
	configDir := session.BuddyConfigDir()
	userID := buddy.GetOrCreateUserID(configDir)
	comp := buddy.LoadCompanion(userID, configDir)
	if comp == nil {
		// Auto-hatch on first launch (no manual /buddy required)
		comp = buddy.HatchWithoutLLM(userID)
		if comp != nil {
			_ = buddy.SaveCompanion(comp, configDir)
		}
	}
	if comp != nil {
		app.SetCompanion(comp)
		app.SetCompanionMuted(buddy.IsCompanionMuted(configDir))

		// P1: Create observer for companion reactions → TUI
		obs := buddy.NewObserver(comp, func(text string) {
			program.Send(tui.CompanionReactionMsg{Text: text})
		})
		runner.SetObserver(obs)
	}

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI: %w", err)
	}
	return nil
}

// wireRunnerCallbacks sets up all runner → TUI callbacks once.
// BUG-7 fix: wire callbacks once at setup time instead of per-submission
// to eliminate the data race on runner callback fields.
func wireRunnerCallbacks(runner *session.Runner, p *tea.Program) {
	runner.OnTextDelta = func(t string) {
		p.Send(tui.StreamTextMsg{Text: t})
	}
	runner.OnToolStart = func(id, name, input string) {
		p.Send(tui.ToolStartMsg{ToolID: id, ToolName: name, Input: input})
	}
	runner.OnToolDone = func(id, output string, isError bool) {
		p.Send(tui.ToolDoneMsg{ToolID: id, Output: output, IsError: isError})
	}
	runner.OnDone = func() {
		p.Send(tui.StreamDoneMsg{})
	}
	runner.OnError = func(err error) {
		p.Send(tui.StreamErrorMsg{Err: err})
	}
	runner.OnSystem = func(t string) {
		p.Send(tui.SystemMsg{Text: t})
	}
	runner.OnClearHistory = func() {
		p.Send(tui.ClearHistoryMsg{})
	}
	runner.OnCompact = func() {
		p.Send(tui.CompactHistoryMsg{})
	}

	// Companion callbacks → TUI state sync
	runner.OnCompanionLoad = func(comp *buddy.Companion) {
		p.Send(tui.CompanionLoadMsg{Companion: comp})
	}
	runner.OnCompanionPet = func(tsMs int64) {
		p.Send(tui.CompanionPetMsg{Timestamp: tsMs})
	}
	runner.OnCompanionMute = func(muted bool) {
		p.Send(tui.CompanionMuteMsg{Muted: muted})
	}
	runner.OnCompanionReaction = func(text string) {
		p.Send(tui.CompanionReactionMsg{Text: text})
	}
}

// handleInteractiveInput processes a user message through the engine runner
// and forwards streaming events back to the TUI via tea.Program.Send.
func handleInteractiveInput(ctx context.Context, runner *session.Runner, p *tea.Program, text string) {
	if p == nil {
		return
	}

	if !runner.HandleInput(ctx, text) {
		p.Send(tea.Quit())
	}
}

// ── Tool event message types (TUI-level) ────────────────────────────────────

// formatToolInput returns a summary of tool input for display.
func formatToolInput(ev *engine.StreamEvent) string {
	if ev.ToolInput == nil {
		return ""
	}
	data, err := json.Marshal(ev.ToolInput)
	if err != nil {
		return fmt.Sprintf("%v", ev.ToolInput)
	}
	return string(data)
}
