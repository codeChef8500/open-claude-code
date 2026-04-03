package powershell

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
			name:    "empty command",
			input:   map[string]interface{}{"command": ""},
			wantErr: "command must not be empty",
		},
		{
			name:    "negative timeout",
			input:   map[string]interface{}{"command": "Get-Date", "timeout": -1},
			wantErr: "timeout must be non-negative",
		},
		{
			name:    "timeout exceeds max",
			input:   map[string]interface{}{"command": "Get-Date", "timeout": 999999999},
			wantErr: "timeout exceeds maximum",
		},
		{
			name:    "dangerous format-volume",
			input:   map[string]interface{}{"command": "Format-Volume -DriveLetter D"},
			wantErr: "dangerous pattern",
		},
		{
			name:    "dangerous stop-computer",
			input:   map[string]interface{}{"command": "Stop-Computer -Force"},
			wantErr: "dangerous pattern",
		},
		{
			name:    "dangerous restart-computer",
			input:   map[string]interface{}{"command": "Restart-Computer"},
			wantErr: "dangerous pattern",
		},
		{
			name:    "valid command",
			input:   map[string]interface{}{"command": "Get-ChildItem -Path ."},
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

func TestIsReadOnlyPSCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"Get-ChildItem -Path .", true},
		{"get-content file.txt", true},
		{"Get-Process", true},
		{"Select-String -Pattern foo", true},
		{"Test-Path /tmp/file", true},
		{"Write-Output hello", true},
		{"Format-Table", true},

		// Non-read-only
		{"Set-Content file.txt -Value hello", false},
		{"Remove-Item file.txt", false},
		{"New-Item -Path ./test", false},
		{"Invoke-WebRequest https://example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isReadOnlyPSCommand(tt.cmd); got != tt.want {
			t.Errorf("isReadOnlyPSCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}
