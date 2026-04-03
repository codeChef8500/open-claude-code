package main

import (
	"context"
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
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
				handleInteractiveInput(ctx, runner, program, text)
			}()
		},
	})
	if err != nil {
		return fmt.Errorf("create TUI: %w", err)
	}

	program = tea.NewProgram(app, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("TUI: %w", err)
	}
	return nil
}

// handleInteractiveInput processes a user message through the engine runner
// and forwards streaming events back to the TUI via tea.Program.Send.
func handleInteractiveInput(ctx context.Context, runner *session.Runner, p *tea.Program, text string) {
	if p == nil {
		return
	}

	// Wire callbacks to send Bubbletea messages.
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
