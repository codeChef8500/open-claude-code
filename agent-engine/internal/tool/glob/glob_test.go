package glob

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
			input:   map[string]interface{}{"pattern": "*.go", "path": `\\server\share`},
			wantErr: "UNC paths are not allowed",
		},
		{
			name:    "valid pattern",
			input:   map[string]interface{}{"pattern": "**/*.go"},
			wantErr: "",
		},
		{
			name:    "valid with path",
			input:   map[string]interface{}{"pattern": "*.ts", "path": "/tmp"},
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
