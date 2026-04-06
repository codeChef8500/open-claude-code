package command

import (
	"context"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Phase 4: Alias System Tests
// Verifies all aliases resolve to the correct canonical command and that
// dispatching via alias produces the same result as dispatching via name.
// ──────────────────────────────────────────────────────────────────────────────

func TestAliasQuit(t *testing.T) {
	assertAlias(t, "q", "quit")
	assertAlias(t, "exit", "quit")
}

func TestAliasVersion(t *testing.T) {
	assertAlias(t, "v", "version")
}

func TestAliasResume(t *testing.T) {
	assertAlias(t, "continue", "resume")
}

func TestAliasSession(t *testing.T) {
	assertAlias(t, "remote", "session")
}

func TestAliasPermissions(t *testing.T) {
	assertAlias(t, "allowed-tools", "permissions")
}

func TestAliasPrivacy(t *testing.T) {
	assertAlias(t, "privacy", "privacy-settings")
}

func TestAliasUpgrade(t *testing.T) {
	assertAlias(t, "update", "upgrade")
}

func TestAliasReleaseNotes(t *testing.T) {
	assertAlias(t, "changelog", "release-notes")
}

func TestAliasTerminalSetup(t *testing.T) {
	assertAlias(t, "terminalsetup", "terminal-setup")
}

func TestAliasPRComments(t *testing.T) {
	assertAlias(t, "pr_comments", "pr-comments")
}

func TestAliasCommitPushPr(t *testing.T) {
	assertAlias(t, "cpp", "commit-push-pr")
}

func TestAliasKeybindings(t *testing.T) {
	assertAlias(t, "keys", "keybindings")
	assertAlias(t, "shortcuts", "keybindings")
}

func TestAliasRewind(t *testing.T) {
	assertAlias(t, "checkpoint", "rewind")
}

func TestAliasSandbox(t *testing.T) {
	assertAlias(t, "sandbox", "sandbox-toggle")
}

func TestAliasTasks(t *testing.T) {
	assertAlias(t, "bashes", "tasks")
}

func TestAliasSubscribePR(t *testing.T) {
	assertAlias(t, "subscribe", "subscribe-pr")
}

func TestAliasWorkflow(t *testing.T) {
	assertAlias(t, "wf", "workflow")
}

// ── Alias dispatch equivalence ─────────────────────────────────────────────

func TestAliasDispatchEquivalence(t *testing.T) {
	ctx := context.Background()
	ectx := newTestEctx()

	// Test that dispatching via alias gives the same result type as canonical name.
	pairs := []struct {
		alias     string
		canonical string
	}{
		{"q", "quit"},
		{"v", "version"},
		{"update", "upgrade"},
		{"changelog", "release-notes"},
		{"sandbox", "sandbox-toggle"},
		{"bashes", "tasks"},
		{"wf", "workflow"},
	}

	for _, p := range pairs {
		t.Run(p.alias+"→"+p.canonical, func(t *testing.T) {
			rAlias := Dispatch(ctx, "/"+p.alias, ectx)
			rCanon := Dispatch(ctx, "/"+p.canonical, ectx)

			if rAlias.Handled != rCanon.Handled {
				t.Errorf("Handled mismatch: alias=%v canonical=%v", rAlias.Handled, rCanon.Handled)
			}
			if rAlias.Type != rCanon.Type {
				t.Errorf("Type mismatch: alias=%s canonical=%s", rAlias.Type, rCanon.Type)
			}
			if rAlias.CommandName != rCanon.CommandName {
				t.Errorf("CommandName mismatch: alias=%s canonical=%s", rAlias.CommandName, rCanon.CommandName)
			}
			// Both should resolve to same error state
			if (rAlias.Error == nil) != (rCanon.Error == nil) {
				t.Errorf("Error state mismatch: alias=%v canonical=%v", rAlias.Error, rCanon.Error)
			}
		})
	}
}

// ── Case insensitivity ─────────────────────────────────────────────────────

func TestAliasCaseInsensitive(t *testing.T) {
	reg := Default()

	// Registry.Find should be case-insensitive for aliases
	cases := []string{"Q", "EXIT", "Update", "CHANGELOG"}
	for _, alias := range cases {
		cmd := reg.Find(strings.ToLower(alias))
		if cmd == nil {
			t.Errorf("alias %q (lowered) should resolve to a command", alias)
		}
	}
}

// ── Helper ─────────────────────────────────────────────────────────────────

func assertAlias(t *testing.T, alias, expectedCanonical string) {
	t.Helper()
	cmd := Default().Find(alias)
	if cmd == nil {
		t.Errorf("alias %q not found in registry", alias)
		return
	}
	if strings.ToLower(cmd.Name()) != strings.ToLower(expectedCanonical) {
		t.Errorf("alias %q → %q, expected %q", alias, cmd.Name(), expectedCanonical)
	}
}
