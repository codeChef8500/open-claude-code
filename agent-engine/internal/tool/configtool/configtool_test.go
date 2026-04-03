package configtool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateInput(t *testing.T) {
	store := NewMapConfigStore(nil)
	tool := New(store)

	tests := []struct {
		name    string
		input   map[string]interface{}
		wantErr string
	}{
		{
			name:    "empty setting",
			input:   map[string]interface{}{"setting": ""},
			wantErr: "setting must not be empty",
		},
		{
			name:    "valid get",
			input:   map[string]interface{}{"setting": "theme"},
			wantErr: "",
		},
		{
			name:    "valid set",
			input:   map[string]interface{}{"setting": "theme", "value": "dark"},
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
