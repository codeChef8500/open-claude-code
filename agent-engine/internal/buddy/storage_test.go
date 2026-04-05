package buddy

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── SaveStoredCompanion / LoadStoredCompanion round-trip ─────────────────────

func TestStorageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sc := &StoredCompanion{
		CompanionSoul: CompanionSoul{
			Name:        "Astra",
			Personality: "calm and wise",
		},
		HatchedAt: 1700000000000,
	}

	if err := SaveStoredCompanion(sc, dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadStoredCompanion(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded nil")
	}
	if loaded.Name != "Astra" {
		t.Errorf("name: %q", loaded.Name)
	}
	if loaded.Personality != "calm and wise" {
		t.Errorf("personality: %q", loaded.Personality)
	}
	if loaded.HatchedAt != 1700000000000 {
		t.Errorf("hatchedAt: %d", loaded.HatchedAt)
	}
}

func TestLoadStoredCompanion_Missing(t *testing.T) {
	dir := t.TempDir()
	sc, err := LoadStoredCompanion(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sc != nil {
		t.Error("expected nil for missing file")
	}
}

func TestLoadStoredCompanion_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, buddyFileName), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadStoredCompanion(dir)
	if err == nil {
		t.Error("expected error for corrupted JSON")
	}
}

// ─── SaveCompanion convenience ───────────────────────────────────────────────

func TestSaveCompanion_Convenience(t *testing.T) {
	dir := t.TempDir()
	comp := &Companion{
		CompanionBones: CompanionBones{
			Species: SpeciesCat,
			Rarity:  RarityRare,
			Eye:     EyeStar,
			Hat:     HatCrown,
			Stats:   map[StatName]int{StatDebugging: 50},
		},
		CompanionSoul: CompanionSoul{
			Name:        "Whiskers",
			Personality: "playful",
		},
		HatchedAt: 1700000000000,
	}

	if err := SaveCompanion(comp, dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Load back and verify only soul + hatchedAt persist
	loaded, err := LoadStoredCompanion(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Name != "Whiskers" {
		t.Errorf("name: %q", loaded.Name)
	}
	if loaded.HatchedAt != 1700000000000 {
		t.Errorf("hatchedAt: %d", loaded.HatchedAt)
	}
}

// ─── LoadCompanion integration ───────────────────────────────────────────────

func TestLoadCompanion_Integration(t *testing.T) {
	dir := t.TempDir()

	// No companion saved yet → nil
	c := LoadCompanion("user-1", dir)
	if c != nil {
		t.Error("expected nil when no companion saved")
	}

	// Save a companion
	sc := &StoredCompanion{
		CompanionSoul: CompanionSoul{Name: "Blobby", Personality: "gooey"},
		HatchedAt:     1700000000000,
	}
	if err := SaveStoredCompanion(sc, dir); err != nil {
		t.Fatal(err)
	}

	// Now load → should merge bones from deterministic roll
	c = LoadCompanion("user-1", dir)
	if c == nil {
		t.Fatal("expected non-nil companion after save")
	}
	if c.Name != "Blobby" {
		t.Errorf("name: %q", c.Name)
	}
	if c.Species == "" {
		t.Error("species should be populated from deterministic roll")
	}
	if len(c.Stats) == 0 {
		t.Error("stats should be populated from deterministic roll")
	}
}

// ─── Mute ────────────────────────────────────────────────────────────────────

func TestMuteRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Initially not muted
	if IsCompanionMuted(dir) {
		t.Error("should not be muted initially")
	}

	// Mute
	if err := SetCompanionMuted(true, dir); err != nil {
		t.Fatalf("mute: %v", err)
	}
	if !IsCompanionMuted(dir) {
		t.Error("should be muted after set true")
	}

	// Unmute
	if err := SetCompanionMuted(false, dir); err != nil {
		t.Fatalf("unmute: %v", err)
	}
	if IsCompanionMuted(dir) {
		t.Error("should not be muted after set false")
	}
}

func TestUnmuteWhenAlreadyUnmuted(t *testing.T) {
	dir := t.TempDir()
	// Should not error when unmuting while already unmuted
	if err := SetCompanionMuted(false, dir); err != nil {
		t.Fatalf("unmute when not muted: %v", err)
	}
}
