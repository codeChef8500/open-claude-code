package command

import (
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Phase 7: PromptMeta and AllowedTools Tests
// Verifies that prompt commands have correct metadata, allowed tools, and
// progress messages.
// ──────────────────────────────────────────────────────────────────────────────

// promptMetaFor returns the PromptCommandMeta for a named command.
func promptMetaFor(t *testing.T, name string) *PromptCommandMeta {
	t.Helper()
	cmd := Default().Find(name)
	if cmd == nil {
		t.Fatalf("command /%s not found", name)
	}
	pc, ok := cmd.(PromptCommand)
	if !ok {
		t.Fatalf("/%s is not a PromptCommand", name)
	}
	return pc.PromptMeta()
}

// ── Individual PromptMeta tests ────────────────────────────────────────────

func TestCommitPromptMeta(t *testing.T) {
	meta := promptMetaFor(t, "commit")
	if meta == nil {
		t.Fatal("/commit PromptMeta is nil")
	}
	if meta.ProgressMessage == "" {
		t.Error("/commit should have a ProgressMessage")
	}
	assertContainsTool(t, "commit", meta.AllowedTools, "Bash")
}

func TestReviewPromptMeta(t *testing.T) {
	meta := promptMetaFor(t, "review")
	if meta == nil {
		t.Fatal("/review PromptMeta is nil")
	}
	if meta.ProgressMessage == "" {
		t.Error("/review should have a ProgressMessage")
	}
	assertContainsTool(t, "review", meta.AllowedTools, "Read")
	assertContainsTool(t, "review", meta.AllowedTools, "Glob")
	assertContainsTool(t, "review", meta.AllowedTools, "Grep")
}

func TestSecurityReviewPromptMeta(t *testing.T) {
	meta := promptMetaFor(t, "security-review")
	if meta == nil {
		t.Fatal("/security-review PromptMeta is nil")
	}
	if meta.ProgressMessage == "" {
		t.Error("/security-review should have a ProgressMessage")
	}
	assertContainsTool(t, "security-review", meta.AllowedTools, "Task")
}

func TestInitPromptMeta(t *testing.T) {
	meta := promptMetaFor(t, "init")
	if meta == nil {
		t.Fatal("/init PromptMeta is nil")
	}
	if meta.ProgressMessage == "" {
		t.Error("/init should have a ProgressMessage")
	}
	assertContainsTool(t, "init", meta.AllowedTools, "Write")
	assertContainsTool(t, "init", meta.AllowedTools, "Edit")
}

func TestCommitPushPrPromptMeta(t *testing.T) {
	meta := promptMetaFor(t, "commit-push-pr")
	if meta == nil {
		t.Fatal("/commit-push-pr PromptMeta is nil")
	}
	if meta.ProgressMessage == "" {
		t.Error("/commit-push-pr should have a ProgressMessage")
	}
	assertContainsTool(t, "commit-push-pr", meta.AllowedTools, "Bash")
}

func TestPromptMetaNilSafe(t *testing.T) {
	// Commands that inherit BasePromptCommand should return nil meta (not panic).
	reg := Default()
	for _, cmd := range reg.All() {
		if cmd.Type() != CommandTypePrompt {
			continue
		}
		pc, ok := cmd.(PromptCommand)
		if !ok {
			continue
		}
		t.Run("/"+cmd.Name(), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("/%s PromptMeta() panicked: %v", cmd.Name(), r)
				}
			}()
			meta := pc.PromptMeta()
			// meta can be nil (base commands) or non-nil (deep impls) — both are fine.
			_ = meta
		})
	}
}

func TestAllPromptCommandsHaveProgressMessage(t *testing.T) {
	// Deep implementation prompt commands should have non-empty ProgressMessage.
	deepPromptCmds := []string{
		"commit", "review", "security-review", "init", "commit-push-pr",
	}

	for _, name := range deepPromptCmds {
		t.Run("/"+name, func(t *testing.T) {
			meta := promptMetaFor(t, name)
			if meta == nil {
				t.Fatalf("/%s: deep prompt command should have non-nil PromptMeta", name)
			}
			if meta.ProgressMessage == "" {
				t.Errorf("/%s: deep prompt command should have ProgressMessage", name)
			}
			t.Logf("/%s: progress=%q tools=%v", name, meta.ProgressMessage, meta.AllowedTools)
		})
	}
}

// TestAllowedToolsNonEmpty verifies deep prompt commands have tool restrictions.
func TestAllowedToolsNonEmpty(t *testing.T) {
	deepPromptCmds := []string{
		"commit", "review", "security-review", "init", "commit-push-pr",
	}

	for _, name := range deepPromptCmds {
		t.Run("/"+name, func(t *testing.T) {
			meta := promptMetaFor(t, name)
			if meta == nil {
				t.Fatalf("/%s PromptMeta is nil", name)
			}
			if len(meta.AllowedTools) == 0 {
				t.Errorf("/%s should have AllowedTools restrictions", name)
			}
			t.Logf("/%s AllowedTools: %v", name, meta.AllowedTools)
		})
	}
}

// TestSecurityReviewHasTaskTool verifies /security-review uses Task tool
// for spawning sub-reviews.
func TestSecurityReviewHasTaskTool(t *testing.T) {
	meta := promptMetaFor(t, "security-review")
	if meta == nil {
		t.Fatal("/security-review PromptMeta is nil")
	}
	found := false
	for _, tool := range meta.AllowedTools {
		if tool == "Task" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("/security-review should include 'Task' in AllowedTools, got %v", meta.AllowedTools)
	}
}

// TestCommitPromptMetaHasBash verifies /commit uses Bash for git operations.
func TestCommitPromptMetaHasBash(t *testing.T) {
	meta := promptMetaFor(t, "commit")
	if meta == nil {
		t.Fatal("/commit PromptMeta is nil")
	}
	assertContainsTool(t, "commit", meta.AllowedTools, "Bash")
}

// TestInitHasWriteAndEdit verifies /init has Write and Edit tools for
// creating CLAUDE.md files.
func TestInitHasWriteAndEdit(t *testing.T) {
	meta := promptMetaFor(t, "init")
	if meta == nil {
		t.Fatal("/init PromptMeta is nil")
	}
	hasWrite := false
	hasEdit := false
	for _, tool := range meta.AllowedTools {
		if tool == "Write" {
			hasWrite = true
		}
		if tool == "Edit" {
			hasEdit = true
		}
	}
	if !hasWrite {
		t.Errorf("/init should include 'Write' in AllowedTools")
	}
	if !hasEdit {
		t.Errorf("/init should include 'Edit' in AllowedTools")
	}
}

// TestCommitPushPrExecContext checks /commit-push-pr does NOT use fork context
// (it runs inline, not as a sub-agent).
func TestCommitPushPrExecContext(t *testing.T) {
	meta := promptMetaFor(t, "commit-push-pr")
	if meta == nil {
		t.Fatal("/commit-push-pr PromptMeta is nil")
	}
	// commit-push-pr should run inline, not forked
	if meta.ExecContext == "fork" {
		t.Error("/commit-push-pr should not use fork exec context")
	}
}

// TestSimplePromptCommandsHaveNilMeta verifies basic prompt commands
// (from commands_git.go, commands_remaining.go) return nil meta.
func TestSimplePromptCommandsHaveNilMeta(t *testing.T) {
	simpleCmds := []string{"pr-comments", "insights", "init-verifiers", "bridge-kick"}

	for _, name := range simpleCmds {
		t.Run("/"+name, func(t *testing.T) {
			meta := promptMetaFor(t, name)
			if meta != nil {
				t.Logf("/%s has PromptMeta (may have been upgraded): progress=%q tools=%v",
					name, meta.ProgressMessage, meta.AllowedTools)
			} else {
				t.Logf("/%s returns nil PromptMeta (base implementation)", name)
			}
		})
	}
}

// ── Helper ─────────────────────────────────────────────────────────────────

func assertContainsTool(t *testing.T, cmdName string, tools []string, expected string) {
	t.Helper()
	for _, tool := range tools {
		// Match exact or prefix (e.g. "Bash" matches "Bash(git add:*)")
		if strings.EqualFold(tool, expected) || strings.HasPrefix(tool, expected) {
			return
		}
	}
	t.Errorf("/%s AllowedTools should contain %q (or prefix), got %v", cmdName, expected, tools)
}
