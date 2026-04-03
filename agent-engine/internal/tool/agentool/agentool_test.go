package agentool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateInput(t *testing.T) {
	tool := New(nil)

	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr string
	}{
		{
			name:    "empty task",
			input:   map[string]interface{}{"task": ""},
			wantErr: "task must not be empty",
		},
		{
			name:    "negative max_turns",
			input:   map[string]interface{}{"task": "do something", "max_turns": -1},
			wantErr: "max_turns must be non-negative",
		},
		{
			name:    "max_turns too high",
			input:   map[string]interface{}{"task": "do something", "max_turns": 300},
			wantErr: "max_turns exceeds maximum",
		},
		{
			name:    "valid task",
			input:   map[string]interface{}{"task": "search for bugs"},
			wantErr: "",
		},
		{
			name:    "valid with max_turns",
			input:   map[string]interface{}{"task": "refactor code", "max_turns": 50},
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
