package command

import (
	"context"
	"testing"
)

// ─── BuddyCommand signal protocol ───────────────────────────────────────────

func TestBuddyCommand_DefaultSignal(t *testing.T) {
	cmd := &BuddyCommand{}
	result, err := cmd.Execute(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "__buddy_show__" {
		t.Errorf("expected __buddy_show__, got %q", result)
	}
}

func TestBuddyCommand_PetSignal(t *testing.T) {
	cmd := &BuddyCommand{}
	result, err := cmd.Execute(context.Background(), []string{"pet"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "__buddy_pet__" {
		t.Errorf("expected __buddy_pet__, got %q", result)
	}
}

func TestBuddyCommand_MuteSignal(t *testing.T) {
	cmd := &BuddyCommand{}
	result, err := cmd.Execute(context.Background(), []string{"mute"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "__buddy_mute__" {
		t.Errorf("expected __buddy_mute__, got %q", result)
	}
}

func TestBuddyCommand_UnmuteSignal(t *testing.T) {
	cmd := &BuddyCommand{}
	result, err := cmd.Execute(context.Background(), []string{"unmute"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "__buddy_unmute__" {
		t.Errorf("expected __buddy_unmute__, got %q", result)
	}
}

func TestBuddyCommand_StatsSignal(t *testing.T) {
	cmd := &BuddyCommand{}
	result, err := cmd.Execute(context.Background(), []string{"stats"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "__buddy_stats__" {
		t.Errorf("expected __buddy_stats__, got %q", result)
	}
}

func TestBuddyCommand_CaseInsensitive(t *testing.T) {
	cmd := &BuddyCommand{}
	result, _ := cmd.Execute(context.Background(), []string{"PET"}, nil)
	if result != "__buddy_pet__" {
		t.Errorf("expected case-insensitive match, got %q", result)
	}
}

func TestBuddyCommand_Metadata(t *testing.T) {
	cmd := &BuddyCommand{}
	if cmd.Name() != "buddy" {
		t.Errorf("name: %q", cmd.Name())
	}
	aliases := cmd.Aliases()
	if len(aliases) != 2 || aliases[0] != "companion" || aliases[1] != "hatch" {
		t.Errorf("aliases: %v", aliases)
	}
	if cmd.Type() != CommandTypeLocal {
		t.Errorf("type: %v", cmd.Type())
	}
	if !cmd.IsEnabled(nil) {
		t.Error("should always be enabled")
	}
}

// ─── Signal parsing ──────────────────────────────────────────────────────────

func TestIsBuddySignal(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"__buddy_show__", true},
		{"__buddy_pet__", true},
		{"__buddy_mute__", true},
		{"__buddy_unmute__", true},
		{"__buddy_stats__", true},
		{"hello world", false},
		{"__hatch__", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsBuddySignal(tt.input)
		if got != tt.want {
			t.Errorf("IsBuddySignal(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseBuddySignal(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"__buddy_show__", "show"},
		{"__buddy_pet__", "pet"},
		{"__buddy_mute__", "mute"},
		{"__buddy_unmute__", "unmute"},
		{"__buddy_stats__", "stats"},
		{"hello", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := ParseBuddySignal(tt.input)
		if got != tt.want {
			t.Errorf("ParseBuddySignal(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ─── Card/Stats text helpers ─────────────────────────────────────────────────

func TestBuddyCardText(t *testing.T) {
	card := BuddyCardText("Astra", "cat", "rare", "★★★", "playful", true, "2026-04-01",
		[]string{"  /\\_/\\  ", " ( · · ) "})
	if card == "" {
		t.Error("card should not be empty")
	}
	if !containsStr(card, "Astra") {
		t.Error("card should contain name")
	}
	if !containsStr(card, "★★★") {
		t.Error("card should contain stars")
	}
	if !containsStr(card, "SHINY") {
		t.Error("card should contain SHINY for shiny companion")
	}
	if !containsStr(card, "2026-04-01") {
		t.Error("card should contain hatch date")
	}
}

func TestBuddyCardText_NotShiny(t *testing.T) {
	card := BuddyCardText("Bob", "blob", "common", "★", "", false, "2026-04-01", nil)
	if containsStr(card, "SHINY") {
		t.Error("non-shiny card should not contain SHINY")
	}
}

func TestBuddyStatsText(t *testing.T) {
	stats := map[string]int{
		"DEBUGGING": 80,
		"PATIENCE":  50,
	}
	text := BuddyStatsText("Astra", "rare", "★★★", "·", "crown", false, stats, []string{"DEBUGGING", "PATIENCE"})
	if text == "" {
		t.Error("stats text should not be empty")
	}
	if !containsStr(text, "Astra") {
		t.Error("stats should contain name")
	}
	if !containsStr(text, "DEBUGGING") {
		t.Error("stats should contain stat name")
	}
}

func TestRenderStatBar(t *testing.T) {
	bar := renderStatBar(50)
	if len(bar) == 0 {
		t.Error("bar should not be empty")
	}
	// 50/5 = 10 filled blocks, 10 empty blocks
	// Using rune count since █ and ░ are multi-byte
	runes := []rune(bar)
	filled := 0
	empty := 0
	for _, r := range runes {
		if r == '█' {
			filled++
		} else if r == '░' {
			empty++
		}
	}
	if filled != 10 {
		t.Errorf("expected 10 filled blocks, got %d", filled)
	}
	if empty != 10 {
		t.Errorf("expected 10 empty blocks, got %d", empty)
	}
}

func TestRenderStatBar_Boundaries(t *testing.T) {
	// Value 0 → 0 filled
	bar0 := renderStatBar(0)
	for _, r := range bar0 {
		if r == '█' {
			t.Error("value 0 should have no filled blocks")
			break
		}
	}

	// Value 100 → 20 filled
	bar100 := renderStatBar(100)
	for _, r := range bar100 {
		if r == '░' {
			t.Error("value 100 should have no empty blocks")
			break
		}
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
