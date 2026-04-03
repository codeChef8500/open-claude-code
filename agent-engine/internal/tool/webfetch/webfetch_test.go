package webfetch

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
			name:    "empty url",
			input:   map[string]interface{}{"url": ""},
			wantErr: "url must not be empty",
		},
		{
			name:    "invalid scheme",
			input:   map[string]interface{}{"url": "ftp://example.com"},
			wantErr: "URL scheme must be http or https",
		},
		{
			name:    "no host",
			input:   map[string]interface{}{"url": "https://"},
			wantErr: "URL must have a host",
		},
		{
			name:    "invalid format",
			input:   map[string]interface{}{"url": "https://example.com", "format": "xml"},
			wantErr: "format must be",
		},
		{
			name:    "valid https",
			input:   map[string]interface{}{"url": "https://example.com"},
			wantErr: "",
		},
		{
			name:    "valid http",
			input:   map[string]interface{}{"url": "http://example.com"},
			wantErr: "",
		},
		{
			name:    "valid with format",
			input:   map[string]interface{}{"url": "https://example.com", "format": "html"},
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
