package command

import (
	"context"
	"testing"
)

// stubCommand is a minimal Command implementation for testing.
type stubCommand struct {
	BaseCommand
	name    string
	aliases []string
	desc    string
	ctype   CommandType
	enabled bool
}

func (s *stubCommand) Name() string                        { return s.name }
func (s *stubCommand) Aliases() []string                   { return s.aliases }
func (s *stubCommand) Description() string                 { return s.desc }
func (s *stubCommand) Type() CommandType                   { return s.ctype }
func (s *stubCommand) IsEnabled(_ *ExecContext) bool       { return s.enabled }
func (s *stubCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "ok", nil
}

func newStub(name, desc string, aliases []string) *stubCommand {
	return &stubCommand{name: name, desc: desc, aliases: aliases, ctype: CommandTypeLocal, enabled: true}
}

func TestRegistry_RegisterAndFind(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("help", "Show help", nil))

	cmd := reg.Find("help")
	if cmd == nil {
		t.Fatal("expected to find 'help' command")
	}
	if cmd.Name() != "help" {
		t.Errorf("expected name 'help', got %q", cmd.Name())
	}
}

func TestRegistry_FindByAlias(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("quit", "Exit", []string{"q", "exit"}))

	cmd := reg.Find("q")
	if cmd == nil {
		t.Fatal("expected to find 'quit' via alias 'q'")
	}
	if cmd.Name() != "quit" {
		t.Errorf("expected name 'quit', got %q", cmd.Name())
	}

	cmd2 := reg.Find("exit")
	if cmd2 == nil {
		t.Fatal("expected to find 'quit' via alias 'exit'")
	}
}

func TestRegistry_FindCaseInsensitive(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("help", "Show help", nil))

	cmd := reg.Find("HELP")
	if cmd == nil {
		t.Fatal("expected case-insensitive find")
	}
}

func TestRegistry_FindNotFound(t *testing.T) {
	reg := NewRegistry()
	cmd := reg.Find("nonexistent")
	if cmd != nil {
		t.Error("expected nil for unknown command")
	}
}

func TestRegistry_IsSlashCommand(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("help", "Show help", nil))

	tests := []struct {
		input string
		want  bool
	}{
		{"/help", true},
		{"/help arg1", true},
		{"/HELP", true},
		{"/unknown", false},
		{"help", false},
		{"", false},
		{"/", false},
	}
	for _, tt := range tests {
		got := reg.IsSlashCommand(tt.input)
		if got != tt.want {
			t.Errorf("IsSlashCommand(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestRegistry_All_Sorted(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("zebra", "z", nil))
	reg.Register(newStub("alpha", "a", nil))
	reg.Register(newStub("mid", "m", nil))

	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	if all[0].Name() != "alpha" {
		t.Errorf("expected first 'alpha', got %q", all[0].Name())
	}
	if all[2].Name() != "zebra" {
		t.Errorf("expected last 'zebra', got %q", all[2].Name())
	}
}

func TestRegistry_Enabled(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("visible", "v", nil))
	disabled := newStub("hidden", "h", nil)
	disabled.enabled = false
	reg.Register(disabled)

	enabled := reg.Enabled(nil)
	if len(enabled) != 1 {
		t.Fatalf("expected 1 enabled, got %d", len(enabled))
	}
	if enabled[0].Name() != "visible" {
		t.Errorf("expected 'visible', got %q", enabled[0].Name())
	}
}

func TestRegistry_RegisterAlias(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("config", "Config", nil))
	reg.RegisterAlias("cfg", "config")

	cmd := reg.Find("cfg")
	if cmd == nil {
		t.Fatal("expected to find 'config' via manual alias 'cfg'")
	}
	if cmd.Name() != "config" {
		t.Errorf("expected 'config', got %q", cmd.Name())
	}
}

func TestRegistry_RegisterPanicsOnDuplicate(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("dup", "first", nil))

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	reg.Register(newStub("dup", "second", nil))
}
