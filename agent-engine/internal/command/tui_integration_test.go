package command

import (
	"context"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Phase 6: TUI Integration Tests
// Simulates the full TUI call chain: input → Dispatch → result → signal routing.
// Also verifies nil-safety and partial ExecContext handling.
// ──────────────────────────────────────────────────────────────────────────────

// TestTUIDispatchFullCycle verifies the complete input → result cycle
// for each command type through the unified Dispatch path.
func TestTUIDispatchFullCycle(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	// Local → plain text output
	r := Dispatch(ctx, "/version", ectx)
	if !r.Handled || r.Error != nil || r.Output == "" {
		t.Errorf("/version full cycle failed: handled=%v err=%v output_len=%d", r.Handled, r.Error, len(r.Output))
	}

	// Interactive → structured result (help is now interactive)
	r = Dispatch(ctx, "/help", ectx)
	if !r.Handled || r.Error != nil || r.Interactive == nil {
		t.Errorf("/help full cycle failed: handled=%v err=%v interactive=%v", r.Handled, r.Error, r.Interactive)
	}

	// Interactive → structured result
	r = Dispatch(ctx, "/model", ectx)
	if !r.Handled || r.Error != nil || r.Interactive == nil {
		t.Errorf("/model full cycle failed: handled=%v err=%v interactive=%v", r.Handled, r.Error, r.Interactive)
	}

	// Prompt → injection text
	r = Dispatch(ctx, "/commit", ectx)
	if !r.Handled || r.Error != nil || r.PromptInjection == "" {
		t.Errorf("/commit full cycle failed: handled=%v err=%v prompt_len=%d", r.Handled, r.Error, len(r.PromptInjection))
	}
}

// TestInteractiveResultRouting verifies that interactive results carry
// the correct component name for TUI routing.
func TestInteractiveResultRouting(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	// Map of command → expected component for TUI routing
	routingTable := map[string]string{
		"/model":       "model",
		"/config":      "config",
		"/mcp":         "mcp",
		"/theme":       "theme",
		"/color":       "color",
		"/keybindings": "keybindings",
		"/export":      "export",
		"/resume":      "resume",
		"/session":     "session",
		"/permissions": "permissions",
		"/memory":      "memory",
		"/plan":        "plan",
		"/effort":      "effort",
		"/fast":        "fast",
		"/agents":      "agents",
		"/tasks":       "tasks",
		"/branch":      "branch",
		"/diff":        "diff",
		"/rewind":      "rewind",
		"/hooks":       "hooks",
		"/feedback":    "feedback",
		"/stats":       "stats",
		"/desktop":     "desktop",
		"/upgrade":     "upgrade",
		"/mobile":      "mobile",
		"/chrome":      "chrome",
		"/ide":         "ide",
	}

	for input, expectedComponent := range routingTable {
		t.Run(input, func(t *testing.T) {
			r := Dispatch(ctx, input, ectx)
			if r.Error != nil {
				t.Fatalf("%s dispatch error: %v", input, r.Error)
			}
			if r.Interactive == nil {
				t.Fatalf("%s: expected interactive result, got nil", input)
			}
			if r.Interactive.Component != expectedComponent {
				t.Errorf("%s: component=%q, expected=%q", input, r.Interactive.Component, expectedComponent)
			}
		})
	}
}

// TestPromptInjectionRouting verifies prompt commands produce non-empty
// injection text through Dispatch.
func TestPromptInjectionRouting(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	promptCmds := []string{
		"/commit", "/review", "/security-review", "/init",
		"/commit-push-pr", "/pr-comments", "/insights", "/init-verifiers",
	}

	for _, input := range promptCmds {
		t.Run(input, func(t *testing.T) {
			r := Dispatch(ctx, input, ectx)
			if r.Error != nil {
				t.Fatalf("%s error: %v", input, r.Error)
			}
			if r.PromptInjection == "" {
				t.Errorf("%s: prompt injection is empty", input)
			}
			if len(r.PromptInjection) < 50 {
				t.Errorf("%s: prompt too short (%d chars)", input, len(r.PromptInjection))
			}
		})
	}
}

// TestQuitSignalHandling verifies /quit produces the __quit__ signal.
func TestQuitSignalHandling(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	r := Dispatch(ctx, "/quit", ectx)
	if r.Output != "__quit__" {
		t.Errorf("/quit should produce '__quit__', got %q", r.Output)
	}

	// Also via alias
	r = Dispatch(ctx, "/q", ectx)
	if r.Output != "__quit__" {
		t.Errorf("/q should produce '__quit__', got %q", r.Output)
	}

	r = Dispatch(ctx, "/exit", ectx)
	if r.Output != "__quit__" {
		t.Errorf("/exit should produce '__quit__', got %q", r.Output)
	}
}

// TestClearSignalHandling verifies /clear produces the __clear_history__ signal.
func TestClearSignalHandling(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	r := Dispatch(ctx, "/clear", ectx)
	if r.Output != "__clear_history__" {
		t.Errorf("/clear should produce '__clear_history__', got %q", r.Output)
	}
}

// TestCompactSignalHandling verifies /compact produces the __compact__ signal.
func TestCompactSignalHandling(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	r := Dispatch(ctx, "/compact", ectx)
	if r.Output != "__compact__" {
		t.Errorf("/compact should produce '__compact__', got %q", r.Output)
	}
}

// TestBuddySignalRouting verifies /buddy signals route correctly.
func TestBuddySignalRouting(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	tests := []struct {
		input  string
		signal string
	}{
		{"/buddy", "__buddy_show__"},
		{"/buddy pet", "__buddy_pet__"},
		{"/buddy mute", "__buddy_mute__"},
		{"/buddy unmute", "__buddy_unmute__"},
		{"/buddy stats", "__buddy_stats__"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			r := Dispatch(ctx, tc.input, ectx)
			if r.Error != nil {
				t.Fatalf("%s error: %v", tc.input, r.Error)
			}
			if r.Output != tc.signal {
				t.Errorf("%s: expected %q, got %q", tc.input, tc.signal, r.Output)
			}
		})
	}
}

// TestCommandWithServicesNil verifies commands don't panic when
// ExecContext.Services is nil.
func TestCommandWithServicesNil(t *testing.T) {
	ctx := context.Background()
	ectx := &ExecContext{
		WorkDir:   "/tmp/test",
		SessionID: "test-session",
		Model:     "test-model",
		// Services is nil
	}

	safeCmds := []string{
		"/help", "/status", "/version", "/quit", "/clear", "/compact",
		"/model", "/theme", "/color", "/keybindings", "/output-style",
		"/agents", "/tasks", "/plan", "/effort", "/fast",
		"/branch", "/diff", "/rewind",
		"/commit", "/review", "/init", "/insights",
	}

	for _, input := range safeCmds {
		t.Run(input, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s panicked with nil Services: %v", input, r)
				}
			}()
			Dispatch(ctx, input, ectx)
		})
	}
}

// TestCommandWithPartialEctx verifies commands handle zero-value fields.
func TestCommandWithPartialEctx(t *testing.T) {
	ctx := context.Background()
	ectx := &ExecContext{} // all zero values

	safeCmds := []string{
		"/help", "/version", "/quit", "/clear", "/compact",
		"/model", "/theme", "/agents", "/tasks",
		"/insights", "/init-verifiers",
	}

	for _, input := range safeCmds {
		t.Run(input, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s panicked with empty ExecContext: %v", input, r)
				}
			}()
			r := Dispatch(ctx, input, ectx)
			if !r.Handled {
				t.Errorf("%s should be handled", input)
			}
		})
	}
}

// TestCommandWithNilEctx verifies Dispatch handles nil ExecContext.
func TestCommandWithNilEctx(t *testing.T) {
	ctx := context.Background()

	// Most commands check IsEnabled(ectx) which may access ectx fields.
	// At minimum, Dispatch itself should not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Dispatch panicked with nil ExecContext: %v", r)
		}
	}()

	// /version and /quit are simple enough to likely work with nil ectx
	r := Dispatch(ctx, "/version", nil)
	if !r.Handled {
		t.Error("/version should be handled even with nil ectx")
	}
}

// TestExecutorAndDispatchConsistency verifies that Executor.Execute and
// Dispatch produce equivalent results for the same input.
func TestExecutorAndDispatchConsistency(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()
	exec := NewExecutor(Default())

	commands := []string{"/help", "/version", "/quit", "/clear", "/compact"}

	for _, input := range commands {
		t.Run(input, func(t *testing.T) {
			// Executor path
			execResult, execErr := exec.Execute(ctx, input, ectx)

			// Dispatch path
			dispResult := Dispatch(ctx, input, ectx)

			// Both should agree on error state
			if (execErr != nil) != (dispResult.Error != nil) {
				t.Errorf("%s: error mismatch — executor=%v dispatch=%v", input, execErr, dispResult.Error)
			}

			// For local commands, both should produce the same output
			if dispResult.Type == CommandTypeLocal && execErr == nil {
				if execResult != dispResult.Output {
					t.Errorf("%s: output mismatch — executor=%q dispatch=%q",
						input,
						truncateStr(execResult, 60),
						truncateStr(dispResult.Output, 60))
				}
			}
		})
	}
}

// TestInteractiveExecutorEncoding verifies the Executor encodes interactive
// results as __interactive__:<component>.
func TestInteractiveExecutorEncoding(t *testing.T) {
	exec := NewExecutor(Default())
	ctx := context.Background()
	ectx := newTestEctx()

	interactiveCmds := []struct {
		input     string
		component string
	}{
		{"/model", "model"},
		{"/config", "config"},
		{"/theme", "theme"},
		{"/agents", "agents"},
	}

	for _, tc := range interactiveCmds {
		t.Run(tc.input, func(t *testing.T) {
			result, err := exec.Execute(ctx, tc.input, ectx)
			if err != nil {
				t.Fatalf("%s executor error: %v", tc.input, err)
			}
			expected := "__interactive__:" + tc.component
			if !strings.HasPrefix(result, expected) {
				t.Errorf("%s: expected prefix %q, got %q", tc.input, expected, result)
			}
		})
	}
}
