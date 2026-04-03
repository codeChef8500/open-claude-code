package fileedit

import (
	"context"
	"encoding/json"
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
			name:    "empty file_path",
			input:   map[string]interface{}{"file_path": "", "old_string": "a", "new_string": "b"},
			wantErr: "file_path must not be empty",
		},
		{
			name:    "no-op edit",
			input:   map[string]interface{}{"file_path": "/tmp/f.go", "old_string": "same", "new_string": "same"},
			wantErr: "old_string and new_string are identical",
		},
		{
			name:    "UNC path",
			input:   map[string]interface{}{"file_path": `\\server\share\f.go`, "old_string": "a", "new_string": "b"},
			wantErr: "UNC paths are not allowed",
		},
		{
			name:    "blocked device path",
			input:   map[string]interface{}{"file_path": "/dev/zero", "old_string": "a", "new_string": "b"},
			wantErr: "cannot edit device file",
		},
		{
			name:    "valid input",
			input:   map[string]interface{}{"file_path": "/tmp/f.go", "old_string": "old", "new_string": "new"},
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
				} else if !contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
