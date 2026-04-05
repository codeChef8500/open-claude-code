package buddy

import (
	"testing"
)

// ─── HatchWithoutLLM ─────────────────────────────────────────────────────────

func TestHatchWithoutLLM_ReturnsCompanion(t *testing.T) {
	c := HatchWithoutLLM("test-user")
	if c == nil {
		t.Fatal("expected non-nil companion")
	}
	if c.Name == "" {
		t.Error("name should not be empty")
	}
	if c.Personality == "" {
		t.Error("personality should not be empty")
	}
	if c.Species == "" {
		t.Error("species should not be empty")
	}
	if c.HatchedAt == 0 {
		t.Error("hatchedAt should be set")
	}
	if len(c.Stats) != len(AllStatNames) {
		t.Errorf("stats count %d != %d", len(c.Stats), len(AllStatNames))
	}
}

func TestHatchWithoutLLM_Deterministic(t *testing.T) {
	c1 := HatchWithoutLLM("deterministic-user")
	c2 := HatchWithoutLLM("deterministic-user")

	// Bones should be identical (deterministic from userID)
	if c1.Species != c2.Species {
		t.Errorf("species mismatch: %s vs %s", c1.Species, c2.Species)
	}
	if c1.Rarity != c2.Rarity {
		t.Errorf("rarity mismatch: %s vs %s", c1.Rarity, c2.Rarity)
	}
	if c1.Eye != c2.Eye {
		t.Errorf("eye mismatch: %s vs %s", c1.Eye, c2.Eye)
	}

	// Soul should also be deterministic (derived from inspiration seed)
	if c1.Name != c2.Name {
		t.Errorf("name mismatch: %s vs %s", c1.Name, c2.Name)
	}
	if c1.Personality != c2.Personality {
		t.Errorf("personality mismatch: %s vs %s", c1.Personality, c2.Personality)
	}
}

func TestHatchWithoutLLM_DifferentUsers(t *testing.T) {
	c1 := HatchWithoutLLM("alice")
	c2 := HatchWithoutLLM("bob")

	// At least some traits should differ
	allSame := c1.Species == c2.Species &&
		c1.Rarity == c2.Rarity &&
		c1.Eye == c2.Eye &&
		c1.Name == c2.Name
	if allSame {
		t.Error("different users produced identical companions (statistically near-impossible)")
	}
}

// ─── defaultSoul ─────────────────────────────────────────────────────────────

func TestDefaultSoul_NameNotEmpty(t *testing.T) {
	bones := CompanionBones{
		Species: SpeciesCat,
		Rarity:  RarityCommon,
	}
	soul := defaultSoul(bones, 12345)
	if soul.Name == "" {
		t.Error("default soul name should not be empty")
	}
	if soul.Personality == "" {
		t.Error("default soul personality should not be empty")
	}
}

func TestDefaultSoul_Deterministic(t *testing.T) {
	bones := CompanionBones{Species: SpeciesDragon}
	s1 := defaultSoul(bones, 42)
	s2 := defaultSoul(bones, 42)
	if s1.Name != s2.Name {
		t.Errorf("name not deterministic: %q vs %q", s1.Name, s2.Name)
	}
	if s1.Personality != s2.Personality {
		t.Errorf("personality not deterministic: %q vs %q", s1.Personality, s2.Personality)
	}
}

func TestDefaultSoul_DifferentSeeds(t *testing.T) {
	bones := CompanionBones{Species: SpeciesDragon}
	s1 := defaultSoul(bones, 1)
	s2 := defaultSoul(bones, 999999)
	// With different seeds, at least one of name/personality should differ
	if s1.Name == s2.Name && s1.Personality == s2.Personality {
		t.Error("different seeds produced identical soul")
	}
}

// ─── capitalise ──────────────────────────────────────────────────────────────

func TestCapitalise(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "Hello"},
		{"Hello", "Hello"},
		{"", ""},
		{"a", "A"},
		{"Z", "Z"},
		{"123", "123"},
	}
	for _, tt := range tests {
		got := capitalise(tt.in)
		if got != tt.want {
			t.Errorf("capitalise(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ─── End-to-end: Hatch → Save → Load ────────────────────────────────────────

func TestHatchSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	userID := "e2e-user"

	// Hatch
	comp := HatchWithoutLLM(userID)
	if comp == nil {
		t.Fatal("hatch returned nil")
	}

	// Save
	if err := SaveCompanion(comp, dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Load
	loaded := LoadCompanion(userID, dir)
	if loaded == nil {
		t.Fatal("load returned nil")
	}

	// Verify soul persisted
	if loaded.Name != comp.Name {
		t.Errorf("name: %q vs %q", loaded.Name, comp.Name)
	}
	if loaded.Personality != comp.Personality {
		t.Errorf("personality: %q vs %q", loaded.Personality, comp.Personality)
	}

	// Verify bones regenerated (should match since same userID)
	if loaded.Species != comp.Species {
		t.Errorf("species: %s vs %s", loaded.Species, comp.Species)
	}
	if loaded.Rarity != comp.Rarity {
		t.Errorf("rarity: %s vs %s", loaded.Rarity, comp.Rarity)
	}
	if loaded.Eye != comp.Eye {
		t.Errorf("eye: %s vs %s", loaded.Eye, comp.Eye)
	}
}
