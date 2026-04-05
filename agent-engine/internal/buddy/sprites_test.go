package buddy

import (
	"strings"
	"testing"
)

// ─── RenderSprite ────────────────────────────────────────────────────────────

func TestRenderSprite_AllSpecies(t *testing.T) {
	for _, sp := range AllSpecies {
		bones := CompanionBones{
			Species: sp,
			Eye:     EyeDot,
			Hat:     HatNone,
		}
		lines := RenderSprite(bones, 0)
		if len(lines) == 0 {
			t.Errorf("species %s: empty sprite", sp)
		}
		for i, line := range lines {
			if strings.Contains(line, "{E}") {
				t.Errorf("species %s frame 0 line %d: unsubstituted {E}", sp, i)
			}
		}
	}
}

func TestRenderSprite_EyeSubstitution(t *testing.T) {
	bones := CompanionBones{
		Species: SpeciesCat,
		Eye:     EyeStar,
		Hat:     HatNone,
	}
	lines := RenderSprite(bones, 0)
	found := false
	for _, line := range lines {
		if strings.Contains(line, string(EyeStar)) {
			found = true
			break
		}
	}
	if !found {
		t.Error("eye character not found in rendered sprite")
	}
}

func TestRenderSprite_HatOverlay(t *testing.T) {
	// Cat with crown: line 0 should be the crown, not blank.
	bones := CompanionBones{
		Species: SpeciesCat,
		Eye:     EyeDot,
		Hat:     HatCrown,
	}
	lines := RenderSprite(bones, 0)
	// The hat should replace the blank line 0
	foundHat := false
	for _, line := range lines {
		if strings.Contains(line, "^^^") { // crown pattern
			foundHat = true
			break
		}
	}
	if !foundHat {
		t.Error("crown hat not found in rendered sprite")
	}
}

func TestRenderSprite_NoHatDropsBlankLine(t *testing.T) {
	// Cat with no hat: if all frames have blank line 0, it should be dropped.
	bones := CompanionBones{
		Species: SpeciesCat,
		Eye:     EyeDot,
		Hat:     HatNone,
	}
	lines := RenderSprite(bones, 0)
	// Cat frames all have blank line 0 → should be 4 lines (5-1)
	if len(lines) != 4 {
		t.Errorf("expected 4 lines (blank hat line dropped), got %d", len(lines))
	}
}

func TestRenderSprite_FrameWrapping(t *testing.T) {
	bones := CompanionBones{
		Species: SpeciesDuck,
		Eye:     EyeDot,
		Hat:     HatNone,
	}
	// Frame 100 should wrap around to frame 100 % 3
	lines1 := RenderSprite(bones, 0)
	lines2 := RenderSprite(bones, 3) // should == frame 0
	if len(lines1) != len(lines2) {
		t.Fatalf("frame wrapping: different line counts: %d vs %d", len(lines1), len(lines2))
	}
	for i := range lines1 {
		if lines1[i] != lines2[i] {
			t.Errorf("frame wrapping: line %d differs", i)
		}
	}
}

func TestRenderSprite_UnknownSpecies(t *testing.T) {
	bones := CompanionBones{
		Species: Species("unicorn"),
		Eye:     EyeDot,
		Hat:     HatNone,
	}
	lines := RenderSprite(bones, 0)
	if len(lines) == 0 {
		t.Error("unknown species should return fallback, not empty")
	}
}

// ─── SpriteFrameCount ────────────────────────────────────────────────────────

func TestSpriteFrameCount_AllSpecies(t *testing.T) {
	for _, sp := range AllSpecies {
		fc := SpriteFrameCount(sp)
		if fc != 3 {
			t.Errorf("species %s: expected 3 frames, got %d", sp, fc)
		}
	}
}

func TestSpriteFrameCount_Unknown(t *testing.T) {
	fc := SpriteFrameCount(Species("unknown"))
	if fc != 1 {
		t.Errorf("unknown species: expected 1 frame, got %d", fc)
	}
}

// ─── RenderFace ──────────────────────────────────────────────────────────────

func TestRenderFace_AllSpecies(t *testing.T) {
	for _, sp := range AllSpecies {
		bones := CompanionBones{
			Species: sp,
			Eye:     EyeDot,
		}
		face := RenderFace(bones)
		if face == "" {
			t.Errorf("species %s: empty face", sp)
		}
		if !strings.Contains(face, string(EyeDot)) {
			t.Errorf("species %s: face doesn't contain eye char", sp)
		}
	}
}

func TestRenderFace_DuckGooseShared(t *testing.T) {
	duckFace := RenderFace(CompanionBones{Species: SpeciesDuck, Eye: EyeDot})
	gooseFace := RenderFace(CompanionBones{Species: SpeciesGoose, Eye: EyeDot})
	// Both should have the same format: (eye>
	if duckFace != gooseFace {
		t.Errorf("duck and goose faces should match: %q vs %q", duckFace, gooseFace)
	}
}

func TestRenderFace_CatHasOmega(t *testing.T) {
	face := RenderFace(CompanionBones{Species: SpeciesCat, Eye: EyeDot})
	if !strings.Contains(face, "ω") {
		t.Errorf("cat face should contain ω: %q", face)
	}
}
