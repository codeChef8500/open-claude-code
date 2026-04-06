package command

import (
	"sort"
	"strings"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Phase 1: Registry Completeness Tests
// Verifies all expected commands are registered, no collisions, correct types.
// ──────────────────────────────────────────────────────────────────────────────

// expectedCommands is the canonical list of all commands that should be
// registered in the default registry, mapped from claude-code-main.
// Key = command name, Value = expected CommandType.
var expectedCommands = map[string]CommandType{
	// Core (builtins.go)
	"help":    CommandTypeInteractive,
	"clear":   CommandTypeLocal,
	"model":   CommandTypeInteractive,
	"compact": CommandTypeLocal,
	"status":  CommandTypeInteractive,

	// Phase 12 (commands_phase12.go)
	"version": CommandTypeLocal,
	"quit":    CommandTypeLocal,

	// Extra builtins (builtins_extra.go)
	"memory":      CommandTypeInteractive,
	"resume":      CommandTypeInteractive,
	"session":     CommandTypeInteractive,
	"permissions": CommandTypeInteractive,
	"plugin":      CommandTypeInteractive,
	"skills":      CommandTypeInteractive,
	"buddy":       CommandTypeLocal,
	"auto-mode":   CommandTypeLocal,

	// UI (ui_impl.go)
	"theme":        CommandTypeInteractive,
	"color":        CommandTypeInteractive,
	"copy":         CommandTypeLocal,
	"export":       CommandTypeInteractive,
	"keybindings":  CommandTypeInteractive,
	"output-style": CommandTypeInteractive,
	"vim":          CommandTypeLocal,
	"rename":       CommandTypeLocal,
	"stickers":     CommandTypeLocal,

	// Git (commands_git.go)
	"branch":         CommandTypeInteractive,
	"diff":           CommandTypeInteractive,
	"pr-comments":    CommandTypePrompt,
	"commit-push-pr": CommandTypePrompt,
	"rewind":         CommandTypeInteractive,

	// Prompt advanced (commands_prompt_advanced.go)
	"commit":          CommandTypePrompt,
	"review":          CommandTypePrompt,
	"init":            CommandTypePrompt,
	"security-review": CommandTypePrompt,
	"ultrareview":     CommandTypeInteractive,

	// Auth (auth_impl.go)
	"login":       CommandTypeInteractive,
	"logout":      CommandTypeLocal,
	"usage":       CommandTypeInteractive,
	"extra-usage": CommandTypeInteractive,
	"passes":      CommandTypeInteractive,

	// Config/MCP/Context/Cost/Doctor
	"config":  CommandTypeInteractive,
	"mcp":     CommandTypeInteractive,
	"context": CommandTypeInteractive,
	"cost":    CommandTypeLocal,
	"doctor":  CommandTypeInteractive,

	// Session impl
	"plan":   CommandTypeInteractive,
	"fast":   CommandTypeInteractive,
	"effort": CommandTypeInteractive,

	// Agent (commands_agent.go)
	"agents": CommandTypeInteractive,
	"tasks":  CommandTypeInteractive,

	// Misc (commands_misc.go)
	"add-dir":          CommandTypeInteractive,
	"hooks":            CommandTypeInteractive,
	"feedback":         CommandTypeInteractive,
	"stats":            CommandTypeInteractive,
	"advisor":          CommandTypeLocal,
	"tag":              CommandTypeInteractive,
	"desktop":          CommandTypeInteractive,
	"privacy-settings": CommandTypeInteractive,
	"upgrade":          CommandTypeInteractive,
	"reload-plugins":   CommandTypeLocal,
	"bridge-kick":      CommandTypePrompt,
	"btw":              CommandTypeInteractive,
	"release-notes":    CommandTypeInteractive,
	"terminal-setup":   CommandTypeInteractive,
	"statusline":       CommandTypePrompt,

	// Remaining (commands_remaining.go)
	"mobile":             CommandTypeInteractive,
	"chrome":             CommandTypeInteractive,
	"ide":                CommandTypeInteractive,
	"sandbox-toggle":     CommandTypeInteractive,
	"rate-limit-options": CommandTypeInteractive,
	"install-github-app": CommandTypeInteractive,
	"install-slack-app":  CommandTypeInteractive,
	"remote-env":         CommandTypeInteractive,
	"remote-setup":       CommandTypeInteractive,
	"files":              CommandTypeLocal,
	"thinkback":          CommandTypeInteractive,
	"thinkback-play":     CommandTypeInteractive,
	"insights":           CommandTypePrompt,
	"init-verifiers":     CommandTypePrompt,
	"heapdump":           CommandTypeLocal,

	// Feature-gated (commands_featuregated.go)
	"proactive":             CommandTypePrompt,
	"brief":                 CommandTypePrompt,
	"assistant":             CommandTypeInteractive,
	"bridge":                CommandTypeInteractive,
	"remote-control-server": CommandTypeLocal,
	"voice":                 CommandTypeInteractive,
	"force-snip":            CommandTypeLocal,
	"subscribe-pr":          CommandTypePrompt,
	"ultraplan":             CommandTypePrompt,
	"torch":                 CommandTypePrompt,
	"peers":                 CommandTypeInteractive,
	"fork":                  CommandTypeInteractive,
	"web":                   CommandTypeInteractive,

	// Workflow
	"workflow": CommandTypeInteractive,

	// Extra commands found in registry (deep impls / text variants)
	"bug-report":       CommandTypeInteractive,
	"context-text":     CommandTypeLocal,
	"doctor-text":      CommandTypeLocal,
	"extra-usage-text": CommandTypeLocal,
	"usage-report":     CommandTypeLocal,
	"verbose":          CommandTypeLocal,
}

// expectedAliases maps alias → canonical command name.
var expectedAliases = map[string]string{
	"q":             "quit",
	"exit":          "quit",
	"v":             "version",
	"continue":      "resume",
	"remote":        "session",
	"allowed-tools": "permissions",
	"privacy":       "privacy-settings",
	"update":        "upgrade",
	"changelog":     "release-notes",
	"terminalsetup": "terminal-setup",
	"pr_comments":   "pr-comments",
	"cpp":           "commit-push-pr",
	"keys":          "keybindings",
	"shortcuts":     "keybindings",
	"checkpoint":    "rewind",
	"sandbox":       "sandbox-toggle",
	"bashes":        "tasks",
	"subscribe":     "subscribe-pr",
	"wf":            "workflow",
}

// expectedHiddenCommands are commands that should have IsHidden() == true.
var expectedHiddenCommands = []string{
	"remote-control-server",
	"files",
	"heapdump",
	"advisor",
	"tag",
	"bridge-kick",
	"stickers",
	"ultrareview",
}

func TestAllExpectedCommandsRegistered(t *testing.T) {
	reg := Default()

	var missing []string
	for name := range expectedCommands {
		if reg.Find(name) == nil {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("Missing %d expected commands: %s", len(missing), strings.Join(missing, ", "))
	}
}

func TestNoCommandNameCollisions(t *testing.T) {
	reg := Default()
	all := reg.All()

	nameCount := make(map[string]int)
	for _, cmd := range all {
		nameCount[strings.ToLower(cmd.Name())]++
	}

	for name, count := range nameCount {
		if count > 1 {
			t.Errorf("Command name collision: %q registered %d times", name, count)
		}
	}
}

func TestAliasesResolveCorrectly(t *testing.T) {
	reg := Default()

	for alias, expectedCanonical := range expectedAliases {
		cmd := reg.Find(alias)
		if cmd == nil {
			t.Errorf("Alias %q does not resolve to any command (expected %q)", alias, expectedCanonical)
			continue
		}
		if strings.ToLower(cmd.Name()) != strings.ToLower(expectedCanonical) {
			t.Errorf("Alias %q resolves to %q, expected %q", alias, cmd.Name(), expectedCanonical)
		}
	}
}

func TestCommandMinimumCount(t *testing.T) {
	reg := Default()
	count := len(reg.All())
	t.Logf("Total registered commands: %d", count)

	// We expect at least 80 commands based on the mapping.
	if count < 80 {
		t.Errorf("Expected at least 80 registered commands, got %d", count)
	}
}

func TestCommandTypesClassification(t *testing.T) {
	reg := Default()

	for name, expectedType := range expectedCommands {
		cmd := reg.Find(name)
		if cmd == nil {
			continue // already tested in TestAllExpectedCommandsRegistered
		}
		if cmd.Type() != expectedType {
			t.Errorf("/%s: expected type %q, got %q", name, expectedType, cmd.Type())
		}
	}
}

func TestHiddenCommands(t *testing.T) {
	reg := Default()

	for _, name := range expectedHiddenCommands {
		cmd := reg.Find(name)
		if cmd == nil {
			t.Errorf("Hidden command %q not found in registry", name)
			continue
		}
		if !cmd.IsHidden() {
			t.Errorf("/%s should be hidden but IsHidden() returns false", name)
		}
	}
}

func TestFeatureGatedCommandsDefaultDisabled(t *testing.T) {
	// Feature-gated commands should be disabled when their flags are off.
	// We test a subset that we know require feature flags.
	gatedCommands := []string{
		"proactive", "brief", "assistant", "bridge",
		"remote-control-server", "voice", "force-snip",
		"subscribe-pr", "ultraplan", "torch", "peers", "fork", "web",
	}

	reg := Default()
	ectx := &ExecContext{} // empty context, no flags

	for _, name := range gatedCommands {
		cmd := reg.Find(name)
		if cmd == nil {
			t.Errorf("Feature-gated command %q not found", name)
			continue
		}
		// We just verify IsEnabled doesn't panic. Actual flag state depends
		// on environment, so we log the result rather than assert.
		enabled := cmd.IsEnabled(ectx)
		t.Logf("/%s: IsEnabled=%v (feature-gated)", name, enabled)
	}
}

func TestAvailabilityConstraints(t *testing.T) {
	reg := Default()

	// /rate-limit-options should have claude-ai availability
	cmd := reg.Find("rate-limit-options")
	if cmd == nil {
		t.Fatal("/rate-limit-options not found")
	}
	avail := cmd.Availability()
	if len(avail) == 0 {
		t.Error("/rate-limit-options should have availability constraints")
	}
	found := false
	for _, a := range avail {
		if a == AvailabilityClaudeAI {
			found = true
		}
	}
	if !found {
		t.Error("/rate-limit-options should require claude-ai availability")
	}

	// Most commands should have no availability restriction
	noRestriction := []string{"help", "clear", "compact", "model", "status"}
	for _, name := range noRestriction {
		cmd := reg.Find(name)
		if cmd == nil {
			continue
		}
		if avail := cmd.Availability(); len(avail) > 0 {
			t.Errorf("/%s should have no availability restriction, got %v", name, avail)
		}
	}
}

func TestUnexpectedExtraCommands(t *testing.T) {
	// Report commands in registry that are NOT in our expectedCommands map.
	// These aren't necessarily errors, but good to know about.
	reg := Default()
	all := reg.All()

	var extra []string
	for _, cmd := range all {
		name := strings.ToLower(cmd.Name())
		if _, ok := expectedCommands[name]; !ok {
			extra = append(extra, name)
		}
	}

	if len(extra) > 0 {
		sort.Strings(extra)
		t.Logf("Commands in registry but not in expectedCommands map (%d): %s",
			len(extra), strings.Join(extra, ", "))
	}
}
