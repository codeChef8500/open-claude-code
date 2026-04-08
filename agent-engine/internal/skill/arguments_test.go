package skill

import (
	"strings"
	"testing"
)

func TestParseArguments(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"  ", nil},
		{"foo bar baz", []string{"foo", "bar", "baz"}},
		{`foo "hello world" baz`, []string{"foo", "hello world", "baz"}},
		{`foo 'hello world' baz`, []string{"foo", "hello world", "baz"}},
	}
	for _, tt := range tests {
		got := ParseArguments(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("ParseArguments(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ParseArguments(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseArgumentNames(t *testing.T) {
	got := ParseArgumentNames([]string{"foo", "123", "bar", "", "baz"})
	want := []string{"foo", "bar", "baz"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGenerateProgressiveArgumentHint(t *testing.T) {
	got := GenerateProgressiveArgumentHint([]string{"file", "flags", "mode"}, []string{"main.go"})
	if got != "[flags] [mode]" {
		t.Errorf("got %q, want %q", got, "[flags] [mode]")
	}
	got = GenerateProgressiveArgumentHint([]string{"file"}, []string{"main.go"})
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSubstituteArguments_FullString(t *testing.T) {
	content := "Run: $ARGUMENTS"
	got := SubstituteArguments(content, "foo bar", true, nil)
	if got != "Run: foo bar" {
		t.Errorf("got %q", got)
	}
}

func TestSubstituteArguments_Indexed(t *testing.T) {
	content := "File: $ARGUMENTS[0], Mode: $ARGUMENTS[1]"
	got := SubstituteArguments(content, "main.go debug", true, nil)
	if got != "File: main.go, Mode: debug" {
		t.Errorf("got %q", got)
	}
}

func TestSubstituteArguments_Shorthand(t *testing.T) {
	content := "File: $0, Mode: $1"
	got := SubstituteArguments(content, "main.go debug", true, nil)
	if got != "File: main.go, Mode: debug" {
		t.Errorf("got %q", got)
	}
}

func TestSubstituteArguments_Named(t *testing.T) {
	content := "File: $file, Flags: $flags"
	got := SubstituteArguments(content, "main.go --verbose", true, []string{"file", "flags"})
	if got != "File: main.go, Flags: --verbose" {
		t.Errorf("got %q", got)
	}
}

func TestSubstituteArguments_AppendIfNoPlaceholder(t *testing.T) {
	content := "Do something"
	got := SubstituteArguments(content, "extra stuff", true, nil)
	if !strings.Contains(got, "ARGUMENTS: extra stuff") {
		t.Errorf("expected appended args, got %q", got)
	}
}

func TestSubstituteArguments_NoAppendIfFalse(t *testing.T) {
	content := "Do something"
	got := SubstituteArguments(content, "extra stuff", false, nil)
	if got != content {
		t.Errorf("expected unchanged content, got %q", got)
	}
}

func TestSubstituteArguments_EmptyArgs(t *testing.T) {
	content := "Do $ARGUMENTS"
	got := SubstituteArguments(content, "", true, nil)
	if got != content {
		t.Errorf("expected unchanged for empty args, got %q", got)
	}
}
