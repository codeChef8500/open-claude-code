package buddy

import (
	"strings"
	"testing"
)

func TestCompanionIntroText_ContainsName(t *testing.T) {
	text := CompanionIntroText("Astra", "cat")
	if !strings.Contains(text, "Astra") {
		t.Error("intro should contain companion name")
	}
	if !strings.Contains(text, "cat") {
		t.Error("intro should contain species")
	}
	if !strings.Contains(text, "# Companion") {
		t.Error("intro should start with # Companion heading")
	}
}

func TestCompanionIntroText_NameAppearsMultipleTimes(t *testing.T) {
	text := CompanionIntroText("Blobby", "blob")
	count := strings.Count(text, "Blobby")
	// Name appears in: "named Blobby", "You're not Blobby", "addresses Blobby",
	// "you're not Blobby", "what Blobby might say"
	if count < 4 {
		t.Errorf("name should appear at least 4 times, found %d", count)
	}
}

func TestShouldInjectIntro_NoCompanion(t *testing.T) {
	text, ok := ShouldInjectIntro(nil, nil)
	if ok || text != "" {
		t.Error("should not inject for nil companion")
	}
}

func TestShouldInjectIntro_EmptyName(t *testing.T) {
	comp := &Companion{
		CompanionSoul: CompanionSoul{Name: ""},
	}
	text, ok := ShouldInjectIntro(comp, nil)
	if ok || text != "" {
		t.Error("should not inject for empty name")
	}
}

func TestShouldInjectIntro_FirstTime(t *testing.T) {
	comp := &Companion{
		CompanionBones: CompanionBones{Species: SpeciesCat},
		CompanionSoul:  CompanionSoul{Name: "Whiskers"},
	}
	text, ok := ShouldInjectIntro(comp, nil)
	if !ok {
		t.Error("should inject for first time")
	}
	if !strings.Contains(text, "Whiskers") {
		t.Error("intro should contain name")
	}
}

func TestShouldInjectIntro_AlreadyAnnounced(t *testing.T) {
	comp := &Companion{
		CompanionBones: CompanionBones{Species: SpeciesCat},
		CompanionSoul:  CompanionSoul{Name: "Whiskers"},
	}
	existingIntros := []string{"Whiskers"}
	text, ok := ShouldInjectIntro(comp, existingIntros)
	if ok || text != "" {
		t.Error("should not inject if already announced")
	}
}

func TestShouldInjectIntro_DifferentNameNotBlocked(t *testing.T) {
	comp := &Companion{
		CompanionBones: CompanionBones{Species: SpeciesCat},
		CompanionSoul:  CompanionSoul{Name: "Whiskers"},
	}
	existingIntros := []string{"OtherBuddy"}
	_, ok := ShouldInjectIntro(comp, existingIntros)
	if !ok {
		t.Error("should inject when existing intros don't match current name")
	}
}
