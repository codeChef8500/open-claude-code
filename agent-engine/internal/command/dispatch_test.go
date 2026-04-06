package command

import (
	"context"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Phase 2: Dispatch Mechanism Tests
// Verifies Dispatch() and Executor routing, fuzzy matching, tab completion.
// ──────────────────────────────────────────────────────────────────────────────

func newTestEctx() *ExecContext {
	return &ExecContext{
		WorkDir:        "/tmp/test",
		SessionID:      "test-session-001",
		Model:          "claude-sonnet-4-20250514",
		AutoMode:       false,
		Verbose:        false,
		PermissionMode: "default",
		CostUSD:        0.42,
		TurnCount:      7,
		TotalTokens:    15000,
		EffortLevel:    "medium",
		Theme:          "dark",
	}
}

// ── Dispatch routing by command type ────────────────────────────────────────

func TestDispatchLocalCommand(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	localCmds := []struct {
		input    string
		contains string // expected substring in output
	}{
		{"/version", "Agent Engine"},
		{"/cost", ""},    // cost returns something, just not empty
		{"/verbose", ""}, // verbose returns text
	}

	for _, tc := range localCmds {
		t.Run(tc.input, func(t *testing.T) {
			r := Dispatch(ctx, tc.input, ectx)
			if !r.Handled {
				t.Fatalf("expected handled=true for %s", tc.input)
			}
			if r.Error != nil {
				t.Fatalf("unexpected error for %s: %v", tc.input, r.Error)
			}
			if r.Type != CommandTypeLocal {
				t.Errorf("%s: expected type local, got %s", tc.input, r.Type)
			}
			if tc.contains != "" && !strings.Contains(r.Output, tc.contains) {
				t.Errorf("%s: output should contain %q, got %q", tc.input, tc.contains, truncateStr(r.Output, 120))
			}
		})
	}
}

func TestDispatchInteractiveCommand(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	interactiveCmds := []struct {
		input     string
		component string
	}{
		{"/help", "help"},
		{"/status", "status"},
		{"/model", "model"},
		{"/config", "config"},
		{"/mcp", "mcp"},
		{"/theme", "theme"},
		{"/color", "color"},
		{"/keybindings", "keybindings"},
		{"/permissions", "permissions"},
		{"/memory", "memory"},
		{"/agents", "agents"},
		{"/tasks", "tasks"},
		{"/plan", "plan"},
		{"/effort", "effort"},
		{"/export", "export"},
	}

	for _, tc := range interactiveCmds {
		t.Run(tc.input, func(t *testing.T) {
			r := Dispatch(ctx, tc.input, ectx)
			if !r.Handled {
				t.Fatalf("expected handled=true for %s", tc.input)
			}
			if r.Error != nil {
				t.Fatalf("unexpected error for %s: %v", tc.input, r.Error)
			}
			if r.Type != CommandTypeInteractive {
				t.Errorf("%s: expected type interactive, got %s", tc.input, r.Type)
			}
			if r.Interactive == nil {
				t.Fatalf("%s: InteractiveResult is nil", tc.input)
			}
			if r.Interactive.Component != tc.component {
				t.Errorf("%s: expected component %q, got %q", tc.input, tc.component, r.Interactive.Component)
			}
		})
	}
}

func TestDispatchPromptCommand(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	promptCmds := []struct {
		input    string
		contains string
	}{
		{"/commit", "Git Safety Protocol"},
		{"/review", "Code Review"},
		{"/security-review", "SECURITY CATEGORIES"},
		{"/init", "CLAUDE.md"},
		{"/insights", "Architecture"},
		{"/init-verifiers", "verifiers"},
		{"/pr-comments", "PR Review Comments"},
	}

	for _, tc := range promptCmds {
		t.Run(tc.input, func(t *testing.T) {
			r := Dispatch(ctx, tc.input, ectx)
			if !r.Handled {
				t.Fatalf("expected handled=true for %s", tc.input)
			}
			if r.Error != nil {
				t.Fatalf("unexpected error for %s: %v", tc.input, r.Error)
			}
			if r.Type != CommandTypePrompt {
				t.Errorf("%s: expected type prompt, got %s", tc.input, r.Type)
			}
			if tc.contains != "" && !strings.Contains(r.PromptInjection, tc.contains) {
				t.Errorf("%s: prompt should contain %q, got (len=%d)", tc.input, tc.contains, len(r.PromptInjection))
			}
		})
	}
}

// ── Immediate command flag ─────────────────────────────────────────────────

func TestDispatchImmediateCommand(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	immediateCmds := []string{"/quit", "/hooks"}

	for _, input := range immediateCmds {
		t.Run(input, func(t *testing.T) {
			r := Dispatch(ctx, input, ectx)
			if !r.Handled {
				t.Fatalf("expected handled=true for %s", input)
			}
			if !r.Immediate {
				t.Errorf("%s: expected Immediate=true", input)
			}
		})
	}

	// Non-immediate commands
	nonImmediateCmds := []string{"/help", "/model", "/commit"}
	for _, input := range nonImmediateCmds {
		t.Run(input+"_not_immediate", func(t *testing.T) {
			r := Dispatch(ctx, input, ectx)
			if r.Immediate {
				t.Errorf("%s: expected Immediate=false", input)
			}
		})
	}
}

// ── Error cases ────────────────────────────────────────────────────────────

func TestDispatchUnknownCommand(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	r := Dispatch(ctx, "/nonexistent-command-xyz", ectx)
	if !r.Handled {
		t.Error("unknown command should still be Handled=true")
	}
	if r.Error == nil {
		t.Error("expected error for unknown command")
	}
	if !strings.Contains(r.Error.Error(), "unknown command") {
		t.Errorf("error should mention 'unknown command', got: %v", r.Error)
	}
}

func TestDispatchNotSlashCommand(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	r := Dispatch(ctx, "just a regular message", ectx)
	if r.Handled {
		t.Error("non-slash input should not be handled")
	}
}

func TestDispatchEmptySlash(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	r := Dispatch(ctx, "/", ectx)
	if r.Handled {
		t.Error("bare slash should not be handled")
	}
}

func TestDispatchDisabledCommand(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	// /files is disabled by default (ant-only)
	r := Dispatch(ctx, "/files", ectx)
	if !r.Handled {
		t.Error("disabled command should still be Handled=true")
	}
	if r.Error == nil {
		t.Error("expected error for disabled command")
	}
}

// ── Fuzzy matching ─────────────────────────────────────────────────────────

func TestFuzzyMatchPrefix(t *testing.T) {
	reg := Default()

	tests := []struct {
		input    string
		expected string
	}{
		{"versio", "version"},      // unique prefix
		{"keybind", "keybindings"}, // unique prefix
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			cmd := fuzzyMatch(reg, tc.input)
			if cmd == nil {
				t.Fatalf("fuzzyMatch(%q) returned nil, expected %q", tc.input, tc.expected)
			}
			if cmd.Name() != tc.expected {
				t.Errorf("fuzzyMatch(%q) = %q, want %q", tc.input, cmd.Name(), tc.expected)
			}
		})
	}
}

func TestFuzzyMatchEditDistance(t *testing.T) {
	reg := Default()

	tests := []struct {
		input    string
		expected string
	}{
		{"modek", "model"},    // typo: substitution (distance 1)
		{"versin", "version"}, // typo: missing 'o' (distance 1)
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			cmd := fuzzyMatch(reg, tc.input)
			if cmd == nil {
				t.Fatalf("fuzzyMatch(%q) returned nil, expected %q", tc.input, tc.expected)
			}
			if cmd.Name() != tc.expected {
				t.Errorf("fuzzyMatch(%q) = %q, want %q", tc.input, cmd.Name(), tc.expected)
			}
		})
	}
}

func TestFuzzyMatchAmbiguous(t *testing.T) {
	reg := Default()

	// "co" is ambiguous: compact, config, commit, context, copy, color, cost...
	cmd := fuzzyMatch(reg, "co")
	if cmd != nil {
		t.Errorf("fuzzyMatch(\"co\") should return nil (ambiguous), got %q", cmd.Name())
	}
}

// ── Tab completion ─────────────────────────────────────────────────────────

func TestTabCompletion(t *testing.T) {
	ectx := newTestEctx()

	tests := []struct {
		prefix      string
		mustContain string
	}{
		{"mod", "/model"},
		{"ver", "/version"},
		{"com", "/commit"},
		{"hel", "/help"},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			matches := CompleteCommand(tc.prefix, ectx)
			found := false
			for _, m := range matches {
				if m == tc.mustContain {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("CompleteCommand(%q) should contain %q, got %v", tc.prefix, tc.mustContain, matches)
			}
		})
	}
}

func TestTabCompletionExcludesHidden(t *testing.T) {
	ectx := newTestEctx()

	// Hidden commands should not appear in tab completion
	matches := CompleteCommand("heap", ectx)
	for _, m := range matches {
		if m == "/heapdump" {
			t.Error("hidden command /heapdump should not appear in tab completion")
		}
	}
}

// ── Executor path (alternative dispatch) ────────────────────────────────────

func TestExecutorLocalPath(t *testing.T) {
	exec := NewExecutor(Default())
	ctx := context.Background()
	ectx := newTestEctx()

	result, err := exec.Execute(ctx, "/version", ectx)
	if err != nil {
		t.Fatalf("executor /version error: %v", err)
	}
	if !strings.Contains(result, "Agent Engine") {
		t.Errorf("executor /version should contain 'Agent Engine', got: %s", truncateStr(result, 80))
	}
}

func TestExecutorInteractivePath(t *testing.T) {
	exec := NewExecutor(Default())
	ctx := context.Background()
	ectx := newTestEctx()

	result, err := exec.Execute(ctx, "/model", ectx)
	if err != nil {
		t.Fatalf("executor /model error: %v", err)
	}
	if !strings.HasPrefix(result, "__interactive__:model") {
		t.Errorf("executor /model should return '__interactive__:model', got: %s", result)
	}
}

func TestExecutorPromptPath(t *testing.T) {
	exec := NewExecutor(Default())
	ctx := context.Background()
	ectx := newTestEctx()

	result, err := exec.Execute(ctx, "/commit", ectx)
	if err != nil {
		t.Fatalf("executor /commit error: %v", err)
	}
	if !strings.HasPrefix(result, "__prompt__:") {
		t.Errorf("executor /commit should return '__prompt__:*', got: %s", truncateStr(result, 80))
	}
}

func truncateStr(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
