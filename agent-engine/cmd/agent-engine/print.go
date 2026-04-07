package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/session"
	"github.com/wall-ai/agent-engine/internal/util"
)

// runPrintMode executes a single prompt non-interactively and exits.
// Mirrors claude-code's `--print` / `-p` mode (cli/print.ts).
func runPrintMode(ctx context.Context, appCfg *util.AppConfig, wd, prompt string) error {
	result, err := session.Bootstrap(ctx, session.BootstrapConfig{
		AppConfig: appCfg,
		WorkDir:   wd,
	})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	defer session.Shutdown(result)

	runner := session.NewRunner(result)

	outputFmt := appCfg.OutputFormat
	if outputFmt == "" {
		outputFmt = "text"
	}

	switch outputFmt {
	case "json":
		return runPrintJSON(ctx, runner, prompt)
	case "stream-json":
		return runPrintStreamJSON(ctx, runner, prompt)
	default:
		return runPrintText(ctx, runner, prompt)
	}
}

// runPrintText outputs plain text to stdout (default).
func runPrintText(ctx context.Context, runner *session.Runner, prompt string) error {
	runner.OnTextDelta = func(text string) {
		fmt.Print(text)
	}
	runner.OnToolStart = func(id, name, input string) {
		fmt.Fprintf(os.Stderr, "\n⚙ %s %s\n", name, input)
	}
	runner.OnToolDone = func(id, name, output string, isError bool) {
		if isError {
			fmt.Fprintf(os.Stderr, "✗ tool error: %s\n", output)
		}
	}
	runner.OnDone = func() {
		fmt.Println()
	}
	runner.OnError = func(err error) {
		fmt.Fprintf(os.Stderr, "⚠ %v\n", err)
	}
	runner.OnSystem = func(text string) {
		fmt.Fprintf(os.Stderr, "▶ %s\n", text)
	}

	runner.HandleInput(ctx, prompt)
	return nil
}

// streamEvent is a single event in stream-json output format.
type streamEvent struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// runPrintStreamJSON outputs newline-delimited JSON events to stdout.
func runPrintStreamJSON(ctx context.Context, runner *session.Runner, prompt string) error {
	enc := json.NewEncoder(os.Stdout)

	runner.OnTextDelta = func(text string) {
		_ = enc.Encode(streamEvent{Type: "text_delta", Content: text})
	}
	runner.OnToolStart = func(id, name, input string) {
		_ = enc.Encode(streamEvent{Type: "tool_start", Content: map[string]string{
			"id": id, "name": name, "input": input,
		}})
	}
	runner.OnToolDone = func(id, name, output string, isError bool) {
		_ = enc.Encode(streamEvent{Type: "tool_done", Content: map[string]interface{}{
			"id": id, "name": name, "output": output, "is_error": isError,
		}})
	}
	runner.OnDone = func() {
		_ = enc.Encode(streamEvent{Type: "done"})
	}
	runner.OnError = func(err error) {
		_ = enc.Encode(streamEvent{Type: "error", Error: err.Error()})
	}
	runner.OnSystem = func(text string) {
		_ = enc.Encode(streamEvent{Type: "system", Content: text})
	}

	runner.HandleInput(ctx, prompt)
	return nil
}

// jsonResult is the final JSON output for --output-format=json.
type jsonResult struct {
	Role    string             `json:"role"`
	Content string             `json:"content"`
	Model   string             `json:"model,omitempty"`
	Usage   *engine.UsageStats `json:"usage,omitempty"`
}

// runPrintJSON collects the full response and outputs a single JSON object.
func runPrintJSON(ctx context.Context, runner *session.Runner, prompt string) error {
	var fullText string
	var lastUsage *engine.UsageStats

	runner.OnTextDelta = func(text string) {
		fullText += text
	}
	runner.OnToolStart = func(id, name, input string) {}
	runner.OnToolDone = func(id, name, output string, isError bool) {}
	runner.OnDone = func() {}
	runner.OnError = func(err error) {
		fmt.Fprintf(os.Stderr, "⚠ %v\n", err)
	}
	runner.OnSystem = func(text string) {}

	// Capture usage from the event stream via a custom callback approach.
	// The runner fires OnDone after draining, so fullText is complete by then.
	origDone := runner.OnDone
	runner.OnDone = func() {
		origDone()
	}

	runner.HandleInput(ctx, prompt)

	result := jsonResult{
		Role:    "assistant",
		Content: fullText,
		Usage:   lastUsage,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
