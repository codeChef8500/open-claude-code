package prompt

import (
	"strings"
	"testing"
)

func TestOutputStyleInstruction(t *testing.T) {
	tests := []struct {
		style OutputStyle
		empty bool
	}{
		{OutputStyleDefault, true},
		{OutputStyleConcise, false},
		{OutputStyleDetailed, false},
		{OutputStyleJSON, false},
		{OutputStyleMarkdown, false},
		{OutputStylePlainText, false},
	}
	for _, tt := range tests {
		instr := OutputStyleInstruction(tt.style)
		if tt.empty && instr != "" {
			t.Errorf("style %q: expected empty, got %q", tt.style, instr)
		}
		if !tt.empty && instr == "" {
			t.Errorf("style %q: expected non-empty instruction", tt.style)
		}
	}
}

func TestInjectOutputStyle_Default(t *testing.T) {
	prompt := "You are a helpful assistant."
	result := InjectOutputStyle(prompt, OutputStyleDefault)
	if result != prompt {
		t.Errorf("default style should return prompt unchanged, got %q", result)
	}
}

func TestInjectOutputStyle_Concise(t *testing.T) {
	prompt := "You are a helpful assistant."
	result := InjectOutputStyle(prompt, OutputStyleConcise)
	if !strings.Contains(result, "## Output Format") {
		t.Error("expected Output Format heading")
	}
	if !strings.Contains(result, "concisely") {
		t.Error("expected concise instruction text")
	}
	if !strings.HasPrefix(result, prompt) {
		t.Error("expected original prompt to be preserved at start")
	}
}

func TestInjectOutputStyle_EmptyPrompt(t *testing.T) {
	result := InjectOutputStyle("", OutputStyleJSON)
	if result == "" {
		t.Error("expected non-empty result for JSON style with empty prompt")
	}
	if strings.Contains(result, "## Output Format") {
		t.Error("empty prompt should not get heading prefix")
	}
}

func TestInjectOutputStyle_TrailingNewlines(t *testing.T) {
	prompt := "System prompt\n\n\n"
	result := InjectOutputStyle(prompt, OutputStyleDetailed)
	// Should trim trailing newlines before appending.
	if strings.Contains(result, "\n\n\n\n") {
		t.Error("should not have excessive trailing newlines")
	}
}
