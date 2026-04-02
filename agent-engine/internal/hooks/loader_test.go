package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHooksFromSettings_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	content := `{
		"hooks": {
			"PreToolUse": [
				{"command": "echo", "args": ["hello"], "timeout_seconds": 10}
			]
		}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	settings, err := LoadHooksFromSettings(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hooks, ok := settings[EventPreToolUse]
	if !ok {
		t.Fatal("expected PreToolUse hooks")
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0].Command != "echo" {
		t.Errorf("expected command 'echo', got %q", hooks[0].Command)
	}
}

func TestLoadHooksFromSettings_NotFound(t *testing.T) {
	settings, err := LoadHooksFromSettings("/nonexistent/path.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if settings != nil {
		t.Error("expected nil settings for missing file")
	}
}

func TestLoadHooksFromSettings_NoHooksKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"other": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadHooksFromSettings(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings != nil {
		t.Error("expected nil settings when no hooks key")
	}
}

func TestMergeHooksSettings(t *testing.T) {
	a := HooksSettings{
		EventPreToolUse: {{Command: "a"}},
	}
	b := HooksSettings{
		EventPreToolUse:  {{Command: "b"}},
		EventPostToolUse: {{Command: "c"}},
	}
	merged := MergeHooksSettings(a, b)
	if len(merged[EventPreToolUse]) != 2 {
		t.Errorf("expected 2 PreToolUse hooks, got %d", len(merged[EventPreToolUse]))
	}
	if len(merged[EventPostToolUse]) != 1 {
		t.Errorf("expected 1 PostToolUse hook, got %d", len(merged[EventPostToolUse]))
	}
}

func TestValidateHookConfig(t *testing.T) {
	// Valid config.
	errs := ValidateHookConfig(HookConfig{Command: "echo", Event: EventPreToolUse})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}

	// Missing command.
	errs = ValidateHookConfig(HookConfig{Event: EventPreToolUse})
	if len(errs) == 0 {
		t.Error("expected error for missing command")
	}

	// Invalid event.
	errs = ValidateHookConfig(HookConfig{Command: "echo", Event: "bogus"})
	found := false
	for _, e := range errs {
		if e == `unknown event "bogus"` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown event error, got %v", errs)
	}

	// Timeout too large.
	errs = ValidateHookConfig(HookConfig{Command: "echo", Event: EventPreToolUse, TimeoutSeconds: 999})
	if len(errs) == 0 {
		t.Error("expected error for large timeout")
	}
}

func TestValidateSettings(t *testing.T) {
	settings := HooksSettings{
		EventPreToolUse: {
			{Command: "good"},
			{Command: ""},
		},
	}
	issues := ValidateSettings(settings)
	if issues == nil {
		t.Fatal("expected validation issues")
	}
	if len(issues[EventPreToolUse]) != 1 {
		t.Errorf("expected 1 issue for PreToolUse, got %d", len(issues[EventPreToolUse]))
	}
}
