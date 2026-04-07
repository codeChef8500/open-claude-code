package session

import (
	"testing"
)

func TestParseTitleResponse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid JSON", `{"title": "Fix auth bug"}`, "Fix auth bug"},
		{"JSON with prefix", `Here: {"title": "Setup CI"}`, "Setup CI"},
		{"empty title", `{"title": ""}`, `{"title": ""}`},
		{"no JSON", `Fix auth bug`, "Fix auth bug"},
		{"quoted", `"Fix auth bug"`, "Fix auth bug"},
		{"too long", `{"title": "` + string(make([]byte, 100)) + `"}`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTitleResponse(tt.input)
			if got != tt.want {
				t.Errorf("parseTitleResponse(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFallbackTitle(t *testing.T) {
	title := fallbackTitle(nil)
	if title != "Untitled session" {
		t.Errorf("nil messages should give 'Untitled session', got %q", title)
	}
}
