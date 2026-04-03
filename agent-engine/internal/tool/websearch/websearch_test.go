package websearch

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateInput(t *testing.T) {
	tool := New("", "")

	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr string
	}{
		{
			name:    "empty query",
			input:   map[string]interface{}{"query": ""},
			wantErr: "query must not be empty",
		},
		{
			name:    "negative max_results",
			input:   map[string]interface{}{"query": "test", "max_results": -1},
			wantErr: "max_results must be non-negative",
		},
		{
			name:    "max_results too high",
			input:   map[string]interface{}{"query": "test", "max_results": 100},
			wantErr: "max_results exceeds maximum",
		},
		{
			name:    "valid query",
			input:   map[string]interface{}{"query": "golang concurrency"},
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
