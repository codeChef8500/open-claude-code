package command

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestAllRegisteredCommands exercises every registered command through the executor.
func TestAllRegisteredCommands(t *testing.T) {
	reg := Default()
	exec := NewExecutor(reg)
	ctx := context.Background()
	ectx := &ExecContext{
		WorkDir:        "/tmp/test",
		SessionID:      "test-session",
		Model:          "test-model",
		AutoMode:       false,
		Verbose:        false,
		PermissionMode: "default",
		CostUSD:        0.1234,
		TurnCount:      5,
		TotalTokens:    10000,
	}

	all := reg.All()
	t.Logf("Total registered commands: %d", len(all))

	for _, cmd := range all {
		name := cmd.Name()
		t.Run("/"+name, func(t *testing.T) {
			input := "/" + name
			result, err := exec.Execute(ctx, input, ectx)
			if err != nil {
				t.Logf("/%s returned error: %v", name, err)
				// Some commands may error without full context — that's OK
				return
			}

			// Classify the result
			switch {
			case result == "":
				t.Logf("/%s → (empty)", name)
			case result == "__quit__":
				t.Logf("/%s → __quit__", name)
			case result == "__clear_history__":
				t.Logf("/%s → __clear_history__", name)
			case result == "__compact__":
				t.Logf("/%s → __compact__", name)
			case strings.HasPrefix(result, "__interactive__:"):
				component := strings.TrimPrefix(result, "__interactive__:")
				t.Logf("/%s → interactive:%s", name, component)
			case strings.HasPrefix(result, "__prompt__:"):
				content := strings.TrimPrefix(result, "__prompt__:")
				t.Logf("/%s → prompt (len=%d)", name, len(content))
			case strings.HasPrefix(result, "__fork_prompt__:"):
				content := strings.TrimPrefix(result, "__fork_prompt__:")
				t.Logf("/%s → fork_prompt (len=%d)", name, len(content))
			default:
				// Plain text result
				lines := strings.Split(result, "\n")
				if len(lines) > 3 {
					t.Logf("/%s → text (%d lines): %s...", name, len(lines), lines[0])
				} else {
					t.Logf("/%s → text: %s", name, result)
				}
			}
		})
	}
}

// TestCommandTypes verifies that each command type dispatches correctly.
func TestCommandTypes(t *testing.T) {
	reg := Default()
	exec := NewExecutor(reg)
	ctx := context.Background()
	ectx := &ExecContext{
		WorkDir:   "/tmp/test",
		SessionID: "test-session",
		Model:     "test-model",
	}

	tests := []struct {
		input    string
		wantType string // "text", "interactive", "prompt", "quit", "clear", "compact", "error"
	}{
		{"/help", "interactive"},
		{"/status", "interactive"},
		{"/cost", "text"},
		{"/model", "interactive"},
		{"/verbose", "text"},
		{"/auto-mode", "text"},
		{"/clear", "clear"},
		{"/compact", "compact"},
		{"/quit", "quit"},
		{"/agents", "interactive"},
		{"/tasks", "interactive"},
		{"/memory", "interactive"},
		{"/plan", "interactive"},
		{"/fast", "interactive"},
		{"/effort", "interactive"},
		{"/skills", "interactive"},
		{"/config", "interactive"},
		{"/mcp", "interactive"},
		{"/login", "interactive"},
		{"/logout", "text"},
		{"/permissions", "interactive"},
		{"/session", "interactive"},
		{"/resume", "interactive"},
		{"/plugin", "interactive"},
		{"/theme", "interactive"},
		{"/branch", "interactive"},
		{"/diff", "interactive"},
		{"/review", "prompt"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := exec.Execute(ctx, tt.input, ectx)

			switch tt.wantType {
			case "text":
				if err != nil {
					t.Errorf("expected text result, got error: %v", err)
					return
				}
				if strings.HasPrefix(result, "__") {
					t.Errorf("expected plain text, got special: %s", result)
				}
				if result == "" {
					t.Errorf("expected non-empty text result")
				}
				t.Logf("OK: %s → %s", tt.input, truncate(result, 80))

			case "interactive":
				if err != nil {
					t.Errorf("expected interactive result, got error: %v", err)
					return
				}
				if !strings.HasPrefix(result, "__interactive__:") {
					t.Errorf("expected __interactive__:*, got: %s", truncate(result, 80))
				}
				t.Logf("OK: %s → %s", tt.input, result)

			case "quit":
				if err != nil {
					t.Errorf("expected quit, got error: %v", err)
					return
				}
				// /quit is handled as special exit in runner, not by executor
				// executor returns empty for it or handles via LocalCommand
				t.Logf("OK: %s → %s", tt.input, truncate(result, 80))

			case "clear":
				if err != nil {
					t.Errorf("expected clear, got error: %v", err)
					return
				}
				if result != "__clear_history__" {
					t.Errorf("expected __clear_history__, got: %s", result)
				}
				t.Logf("OK: %s → %s", tt.input, result)

			case "compact":
				if err != nil {
					t.Errorf("expected compact, got error: %v", err)
					return
				}
				if result != "__compact__" {
					t.Errorf("expected __compact__, got: %s", result)
				}
				t.Logf("OK: %s → %s", tt.input, result)

			case "prompt":
				if err != nil {
					t.Errorf("expected prompt result, got error: %v", err)
					return
				}
				if !strings.HasPrefix(result, "__prompt__:") {
					t.Errorf("expected __prompt__:*, got: %s", truncate(result, 80))
				}
				t.Logf("OK: %s → prompt (len=%d)", tt.input, len(result))

			case "error":
				if err == nil {
					t.Errorf("expected error, got result: %s", truncate(result, 80))
				}
				t.Logf("OK: %s → error: %v", tt.input, err)
			}
		})
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// TestRegisteredCommandCount verifies we have a reasonable number of commands.
func TestRegisteredCommandCount(t *testing.T) {
	reg := Default()
	all := reg.All()
	count := len(all)
	t.Logf("Registered commands: %d", count)
	if count < 20 {
		t.Errorf("expected at least 20 commands, got %d", count)
	}

	// List them all
	for _, cmd := range all {
		aliases := ""
		if a := cmd.Aliases(); len(a) > 0 {
			aliases = " (aliases: " + strings.Join(a, ", ") + ")"
		}
		fmt.Printf("  /%s — %s [%s]%s\n", cmd.Name(), cmd.Description(), cmd.Type(), aliases)
	}
}
