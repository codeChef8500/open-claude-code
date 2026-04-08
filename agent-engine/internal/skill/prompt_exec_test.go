package skill

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSubstituteVariables(t *testing.T) {
	vars := map[string]string{
		"CLAUDE_SKILL_DIR":   "/home/user/.claude/skills",
		"CLAUDE_SESSION_ID":  "abc-123",
		"CLAUDE_PLUGIN_ROOT": "/plugins/my-plugin",
	}
	content := "Dir: ${CLAUDE_SKILL_DIR}, Session: ${CLAUDE_SESSION_ID}, Plugin: ${CLAUDE_PLUGIN_ROOT}"
	got := SubstituteVariables(content, vars)
	want := "Dir: /home/user/.claude/skills, Session: abc-123, Plugin: /plugins/my-plugin"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstituteVariables_NoVars(t *testing.T) {
	content := "No placeholders here"
	got := SubstituteVariables(content, nil)
	if got != content {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestSubstituteVariables_Empty(t *testing.T) {
	content := "Has ${UNKNOWN} var"
	got := SubstituteVariables(content, map[string]string{"OTHER": "val"})
	if got != content {
		t.Errorf("expected unchanged for unknown var, got %q", got)
	}
}

func TestExecuteShellCommandsInPrompt_NoCommands(t *testing.T) {
	text := "Just normal text with no shell commands"
	got, err := ExecuteShellCommandsInPrompt(text, ShellExecContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != text {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestExecuteShellCommandsInPrompt_Inline(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = `Write-Output "hello"`
	} else {
		cmd = "echo hello"
	}
	text := "Result: !`" + cmd + "`"
	got, err := ExecuteShellCommandsInPrompt(text, ShellExecContext{
		Ctx:     context.Background(),
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("expected 'hello' in output, got %q", got)
	}
}

func TestExecuteShellCommandsInPrompt_Block(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = `Write-Output "world"`
	} else {
		cmd = "echo world"
	}
	text := "Before\n```!\n" + cmd + "\n```\nAfter"
	got, err := ExecuteShellCommandsInPrompt(text, ShellExecContext{
		Ctx:     context.Background(),
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "world") {
		t.Errorf("expected 'world' in output, got %q", got)
	}
	if !strings.Contains(got, "Before") || !strings.Contains(got, "After") {
		t.Errorf("expected surrounding text preserved, got %q", got)
	}
}

func TestResolveShell(t *testing.T) {
	bin, arg := resolveShell("bash")
	if bin != "bash" || arg != "-c" {
		t.Errorf("bash: got %s %s", bin, arg)
	}
	bin, arg = resolveShell("powershell")
	if bin != "powershell" || arg != "-Command" {
		t.Errorf("powershell: got %s %s", bin, arg)
	}
}
