package command

import (
	"context"
	"encoding/json"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Phase 5: InteractiveResult Structure Tests
// Verifies that every InteractiveCommand returns well-formed, JSON-serializable
// InteractiveResult with the expected Component and Data shape.
// ──────────────────────────────────────────────────────────────────────────────

// TestAllInteractiveResultsSerializable iterates over every registered
// InteractiveCommand and verifies that ExecuteInteractive returns a result
// that can be JSON-marshaled without error.
func TestAllInteractiveResultsSerializable(t *testing.T) {
	reg := Default()
	ctx := context.Background()
	ectx := newTestEctx()

	for _, cmd := range reg.All() {
		if cmd.Type() != CommandTypeInteractive {
			continue
		}
		if !cmd.IsEnabled(ectx) {
			continue
		}

		ic, ok := cmd.(InteractiveCommand)
		if !ok {
			continue
		}

		t.Run("/"+cmd.Name(), func(t *testing.T) {
			r, err := ic.ExecuteInteractive(ctx, nil, ectx)
			if err != nil {
				t.Logf("/%s ExecuteInteractive error (may be expected without full context): %v", cmd.Name(), err)
				return
			}
			if r == nil {
				t.Fatalf("/%s returned nil InteractiveResult", cmd.Name())
			}
			if r.Component == "" {
				t.Errorf("/%s InteractiveResult has empty Component", cmd.Name())
			}

			// Marshal to JSON — must not panic or error
			bs, err := json.Marshal(r)
			if err != nil {
				t.Errorf("/%s InteractiveResult JSON marshal error: %v", cmd.Name(), err)
			}
			if len(bs) == 0 {
				t.Errorf("/%s InteractiveResult marshaled to empty JSON", cmd.Name())
			}
			t.Logf("/%s → component=%q json_size=%d", cmd.Name(), r.Component, len(bs))
		})
	}
}

// TestModelPickerDataShape verifies /model returns structured ModelPickerData.
func TestModelPickerDataShape(t *testing.T) {
	r := execInteractive(t, "model", nil, newTestEctx())
	if r.Data == nil {
		t.Fatal("/model should have Data")
	}

	// Marshal and re-parse to verify structure
	bs, err := json.Marshal(r.Data)
	if err != nil {
		t.Fatalf("json.Marshal(model Data): %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(bs, &m); err != nil {
		t.Fatalf("json.Unmarshal(model Data): %v", err)
	}

	if _, ok := m["current_model"]; !ok {
		t.Error("/model Data should have 'current_model' field")
	}
	t.Logf("/model Data keys: %v", mapKeys(m))
}

// TestThemeViewDataShape verifies /theme returns themes list.
func TestThemeViewDataShape(t *testing.T) {
	ectx := newTestEctx()
	ectx.Theme = "dark"
	r := execInteractive(t, "theme", nil, ectx)
	if r.Data == nil {
		t.Fatal("/theme should have Data")
	}

	bs, _ := json.Marshal(r.Data)
	var m map[string]interface{}
	json.Unmarshal(bs, &m)

	themes, ok := m["available_themes"]
	if !ok {
		t.Error("/theme Data should have 'available_themes' field")
	}
	if arr, ok := themes.([]interface{}); ok {
		if len(arr) < 10 {
			t.Errorf("/theme should have >= 10 themes, got %d", len(arr))
		}
		t.Logf("/theme has %d themes", len(arr))
	}
}

// TestColorViewDataShape verifies /color data.
func TestColorViewDataShape(t *testing.T) {
	r := execInteractive(t, "color", []string{"blue"}, newTestEctx())
	if r.Data == nil {
		t.Fatal("/color should have Data")
	}

	bs, _ := json.Marshal(r.Data)
	var m map[string]interface{}
	json.Unmarshal(bs, &m)

	if sel, ok := m["selected_color"]; ok {
		if sel != "blue" {
			t.Errorf("selected_color should be 'blue', got %v", sel)
		}
	} else {
		t.Error("/color Data should have 'selected_color'")
	}
}

// TestExportViewDataShape verifies /export data.
func TestExportViewDataShape(t *testing.T) {
	r := execInteractive(t, "export", []string{"json"}, newTestEctx())
	if r.Data == nil {
		t.Fatal("/export should have Data")
	}

	bs, _ := json.Marshal(r.Data)
	var m map[string]interface{}
	json.Unmarshal(bs, &m)

	if _, ok := m["format"]; !ok {
		t.Error("/export Data should have 'format' field")
	}
	if _, ok := m["session_id"]; !ok {
		t.Error("/export Data should have 'session_id' field")
	}
	t.Logf("/export Data keys: %v", mapKeys(m))
}

// TestKeybindingsViewDataShape verifies /keybindings data.
func TestKeybindingsViewDataShape(t *testing.T) {
	r := execInteractive(t, "keybindings", nil, newTestEctx())
	if r.Data == nil {
		t.Fatal("/keybindings should have Data")
	}

	bs, _ := json.Marshal(r.Data)
	var m map[string]interface{}
	json.Unmarshal(bs, &m)

	bindings, ok := m["bindings"]
	if !ok {
		t.Fatal("/keybindings Data should have 'bindings' field")
	}
	arr, ok := bindings.([]interface{})
	if !ok {
		t.Fatal("/keybindings bindings should be an array")
	}
	if len(arr) < 10 {
		t.Errorf("/keybindings should have >= 10 bindings, got %d", len(arr))
	}

	// Verify binding structure
	if len(arr) > 0 {
		first, ok := arr[0].(map[string]interface{})
		if ok {
			for _, key := range []string{"key", "description", "category"} {
				if _, exists := first[key]; !exists {
					t.Errorf("binding should have '%s' field", key)
				}
			}
		}
	}
	t.Logf("/keybindings has %d bindings", len(arr))
}

// TestOutputStyleViewDataShape verifies /output-style data.
func TestOutputStyleViewDataShape(t *testing.T) {
	r := execInteractive(t, "output-style", nil, newTestEctx())
	if r.Data == nil {
		t.Fatal("/output-style should have Data")
	}

	bs, _ := json.Marshal(r.Data)
	var m map[string]interface{}
	json.Unmarshal(bs, &m)

	avail, ok := m["available"]
	if !ok {
		t.Fatal("/output-style Data should have 'available' field")
	}
	arr, ok := avail.([]interface{})
	if !ok {
		t.Fatal("available should be array")
	}
	if len(arr) < 4 {
		t.Errorf("/output-style should have >= 4 styles, got %d", len(arr))
	}
	t.Logf("/output-style has %d styles", len(arr))
}

// TestResumeViewDataShape verifies /resume data.
func TestResumeViewDataShape(t *testing.T) {
	r := execInteractive(t, "resume", []string{"abc"}, newTestEctx())
	if r.Data == nil {
		t.Fatal("/resume should have Data")
	}

	bs, _ := json.Marshal(r.Data)
	var m map[string]interface{}
	json.Unmarshal(bs, &m)

	if _, ok := m["selected_id"]; !ok {
		t.Error("/resume Data should have 'selected_id'")
	}
	t.Logf("/resume Data keys: %v", mapKeys(m))
}

// TestLoginViewDataShape verifies /login data.
func TestLoginViewDataShape(t *testing.T) {
	r := execInteractive(t, "login", nil, newTestEctx())
	if r.Data == nil {
		t.Fatal("/login should have Data")
	}

	bs, _ := json.Marshal(r.Data)
	var m map[string]interface{}
	json.Unmarshal(bs, &m)

	if _, ok := m["is_authenticated"]; !ok {
		t.Error("/login Data should have 'is_authenticated'")
	}
	t.Logf("/login Data keys: %v", mapKeys(m))
}

// TestBranchDataShape verifies /branch passes name and session ID.
func TestBranchDataShape(t *testing.T) {
	r := execInteractive(t, "branch", []string{"feature-x"}, newTestEctx())
	if r.Data == nil {
		t.Fatal("/branch should have Data")
	}

	bs, err := json.Marshal(r.Data)
	if err != nil {
		t.Fatalf("json.Marshal branch data: %v", err)
	}
	t.Logf("/branch raw JSON: %s", string(bs))

	var m map[string]interface{}
	if err := json.Unmarshal(bs, &m); err != nil {
		t.Fatalf("json.Unmarshal branch data: %v", err)
	}
	t.Logf("/branch Data keys: %v", mapKeys(m))

	if name, ok := m["branch_name"]; ok {
		if name != "feature-x" {
			t.Errorf("branch_name should be 'feature-x', got %v", name)
		}
	} else {
		t.Error("/branch Data should have 'branch_name'")
	}
	if _, ok := m["session_id"]; !ok {
		t.Error("/branch Data should have 'session_id'")
	}
}

// TestAddDirDataShape verifies /add-dir returns text output (now LocalCommand).
func TestAddDirDataShape(t *testing.T) {
	result := execLocal(t, "add-dir", []string{"/home/user/project"}, newTestEctx())
	if result == "" {
		t.Fatal("/add-dir should return non-empty output")
	}
}

// TestBtwDataShape verifies /btw passes message.
func TestBtwDataShape(t *testing.T) {
	r := execInteractive(t, "btw", []string{"hello", "world"}, newTestEctx())
	if r.Data == nil {
		t.Fatal("/btw should have Data")
	}

	bs, _ := json.Marshal(r.Data)
	var m map[string]interface{}
	json.Unmarshal(bs, &m)

	if msg, ok := m["message"]; ok {
		if msg != "hello world" {
			t.Errorf("btw message should be 'hello world', got %v", msg)
		}
	} else {
		t.Error("/btw Data should have 'message'")
	}
}

// ── Helper ─────────────────────────────────────────────────────────────────

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
