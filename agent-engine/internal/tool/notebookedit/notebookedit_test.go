package notebookedit

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
			name:    "empty notebook_path",
			input:   map[string]interface{}{"notebook_path": "", "cell_number": 0, "new_source": "x"},
			wantErr: "notebook_path must not be empty",
		},
		{
			name:    "UNC path",
			input:   map[string]interface{}{"notebook_path": `\\server\share\nb.ipynb`, "cell_number": 0, "new_source": "x"},
			wantErr: "UNC paths are not allowed",
		},
		{
			name:    "wrong extension",
			input:   map[string]interface{}{"notebook_path": "/tmp/file.py", "cell_number": 0, "new_source": "x"},
			wantErr: ".ipynb extension",
		},
		{
			name:    "invalid edit_mode",
			input:   map[string]interface{}{"notebook_path": "/tmp/nb.ipynb", "cell_number": 0, "new_source": "x", "edit_mode": "delete"},
			wantErr: "edit_mode must be",
		},
		{
			name:    "insert without cell_type",
			input:   map[string]interface{}{"notebook_path": "/tmp/nb.ipynb", "cell_number": 0, "new_source": "x", "edit_mode": "insert"},
			wantErr: "cell_type is required",
		},
		{
			name:    "invalid cell_type",
			input:   map[string]interface{}{"notebook_path": "/tmp/nb.ipynb", "cell_number": 0, "new_source": "x", "cell_type": "raw"},
			wantErr: "cell_type must be",
		},
		{
			name:    "negative cell_number",
			input:   map[string]interface{}{"notebook_path": "/tmp/nb.ipynb", "cell_number": -1, "new_source": "x"},
			wantErr: "cell_number must be non-negative",
		},
		{
			name:    "valid replace",
			input:   map[string]interface{}{"notebook_path": "/tmp/nb.ipynb", "cell_number": 0, "new_source": "print('hello')"},
			wantErr: "",
		},
		{
			name:    "valid insert",
			input:   map[string]interface{}{"notebook_path": "/tmp/nb.ipynb", "cell_number": 0, "new_source": "# Title", "edit_mode": "insert", "cell_type": "markdown"},
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
