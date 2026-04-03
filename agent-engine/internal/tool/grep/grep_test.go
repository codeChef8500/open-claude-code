package grep

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateInput(t *testing.T) {
	tool := New()

	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr string
	}{
		{
			name:    "empty pattern",
			input:   map[string]interface{}{"pattern": ""},
			wantErr: "pattern must not be empty",
		},
		{
			name:    "UNC path",
			input:   map[string]interface{}{"pattern": "foo", "path": `\\server\share`},
			wantErr: "UNC paths are not allowed",
		},
		{
			name:    "invalid output_mode",
			input:   map[string]interface{}{"pattern": "foo", "output_mode": "bad"},
			wantErr: "output_mode must be",
		},
		{
			name:    "invalid regex",
			input:   map[string]interface{}{"pattern": "[invalid"},
			wantErr: "invalid regex pattern",
		},
		{
			name:    "valid content mode",
			input:   map[string]interface{}{"pattern": "func.*main", "output_mode": "content"},
			wantErr: "",
		},
		{
			name:    "valid files_with_matches",
			input:   map[string]interface{}{"pattern": "TODO", "output_mode": "files_with_matches"},
			wantErr: "",
		},
		{
			name:    "valid count mode",
			input:   map[string]interface{}{"pattern": "import", "output_mode": "count"},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.input)
			err := tool.ValidateInput(context.Background(), raw)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestApplyPagination(t *testing.T) {
	lines := "line0\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9"

	t.Run("default limit 250", func(t *testing.T) {
		in := Input{Pattern: "x"}
		result := applyPagination(lines, in)
		if result != lines {
			t.Errorf("expected unchanged output with default limit, got truncated")
		}
	})

	t.Run("limit 3", func(t *testing.T) {
		limit := 3
		in := Input{Pattern: "x", HeadLimit: &limit}
		result := applyPagination(lines, in)
		if !strings.Contains(result, "line0") {
			t.Errorf("expected line0 in result")
		}
		if !strings.Contains(result, "results truncated") {
			t.Errorf("expected truncation notice")
		}
	})

	t.Run("offset 5", func(t *testing.T) {
		in := Input{Pattern: "x", Offset: 5}
		result := applyPagination(lines, in)
		if strings.Contains(result, "line0\n") {
			t.Errorf("expected offset to skip early lines")
		}
		if !strings.Contains(result, "line5") {
			t.Errorf("expected line5 in result")
		}
	})

	t.Run("unlimited", func(t *testing.T) {
		zero := 0
		in := Input{Pattern: "x", HeadLimit: &zero}
		result := applyPagination(lines, in)
		if result != lines {
			t.Errorf("expected full output with limit=0")
		}
	})

	t.Run("offset beyond end", func(t *testing.T) {
		in := Input{Pattern: "x", Offset: 100}
		result := applyPagination(lines, in)
		if result != "" {
			t.Errorf("expected empty result for offset beyond end, got %q", result)
		}
	})
}

func TestBuildRipgrepArgs(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		in := Input{Pattern: "hello"}
		args := buildRipgrepArgs(in, "/tmp")
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--smart-case") {
			t.Error("expected --smart-case for nil CaseSensitive")
		}
		if !strings.Contains(joined, "--color=never") {
			t.Error("expected --color=never")
		}
		// VCS exclusion
		if !strings.Contains(joined, "!.git") {
			t.Error("expected .git exclusion")
		}
	})

	t.Run("case sensitive true", func(t *testing.T) {
		cs := true
		in := Input{Pattern: "Foo", CaseSensitive: &cs}
		args := buildRipgrepArgs(in, "/tmp")
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--case-sensitive") {
			t.Error("expected --case-sensitive")
		}
	})

	t.Run("case sensitive false", func(t *testing.T) {
		cs := false
		in := Input{Pattern: "foo", CaseSensitive: &cs}
		args := buildRipgrepArgs(in, "/tmp")
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--ignore-case") {
			t.Error("expected --ignore-case")
		}
	})

	t.Run("multiline", func(t *testing.T) {
		in := Input{Pattern: "struct.*field", Multiline: true}
		args := buildRipgrepArgs(in, "/tmp")
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--multiline") {
			t.Error("expected --multiline flag")
		}
	})

	t.Run("files_with_matches", func(t *testing.T) {
		in := Input{Pattern: "TODO", OutputMode: "files_with_matches"}
		args := buildRipgrepArgs(in, "/tmp")
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--files-with-matches") {
			t.Error("expected --files-with-matches flag")
		}
	})

	t.Run("count mode", func(t *testing.T) {
		in := Input{Pattern: "import", OutputMode: "count"}
		args := buildRipgrepArgs(in, "/tmp")
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--count") {
			t.Error("expected --count flag")
		}
	})

	t.Run("context lines", func(t *testing.T) {
		in := Input{Pattern: "error", ContextLines: 3}
		args := buildRipgrepArgs(in, "/tmp")
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--context=3") {
			t.Error("expected --context=3")
		}
	})

	t.Run("type filter", func(t *testing.T) {
		in := Input{Pattern: "func", Type: "go"}
		args := buildRipgrepArgs(in, "/tmp")
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--type") || !strings.Contains(joined, "go") {
			t.Error("expected --type go")
		}
	})
}
