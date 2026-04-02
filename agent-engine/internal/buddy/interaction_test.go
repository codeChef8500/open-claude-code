package buddy

import (
	"testing"
	"time"
)

func makeTestCompanion() *Companion {
	return &Companion{
		Bones: GenerateBones(12345),
		Soul: CompanionSoul{
			Name:      "TestBuddy",
			Mood:      0.5,
			Energy:    0.5,
			Affection: 0.5,
		},
	}
}

func TestInteractionLog_Apply(t *testing.T) {
	c := makeTestCompanion()
	log := NewInteractionLog()
	log.Cooldown = 0 // disable cooldown for testing

	interaction, err := log.Apply(c, InteractionFeed)
	if err != nil {
		t.Fatalf("Apply feed: %v", err)
	}
	if interaction.Type != InteractionFeed {
		t.Errorf("expected feed, got %s", interaction.Type)
	}
	if c.Soul.Energy <= 0.5 {
		t.Errorf("expected energy increase after feed, got %f", c.Soul.Energy)
	}
	if c.Soul.Mood <= 0.5 {
		t.Errorf("expected mood increase after feed, got %f", c.Soul.Mood)
	}
}

func TestInteractionLog_Play(t *testing.T) {
	c := makeTestCompanion()
	log := NewInteractionLog()
	log.Cooldown = 0

	_, err := log.Apply(c, InteractionPlay)
	if err != nil {
		t.Fatalf("Apply play: %v", err)
	}
	// Play increases mood but decreases energy.
	if c.Soul.Mood <= 0.5 {
		t.Errorf("expected mood increase, got %f", c.Soul.Mood)
	}
	if c.Soul.Energy >= 0.5 {
		t.Errorf("expected energy decrease, got %f", c.Soul.Energy)
	}
	if c.Soul.Affection <= 0.5 {
		t.Errorf("expected affection increase, got %f", c.Soul.Affection)
	}
}

func TestInteractionLog_Cooldown(t *testing.T) {
	c := makeTestCompanion()
	log := NewInteractionLog()
	log.Cooldown = 1 * time.Hour // very long cooldown

	_, err := log.Apply(c, InteractionPet)
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}

	_, err = log.Apply(c, InteractionPet)
	if err == nil {
		t.Error("expected cooldown error on second apply")
	}
}

func TestInteractionLog_UnknownType(t *testing.T) {
	c := makeTestCompanion()
	log := NewInteractionLog()
	log.Cooldown = 0

	_, err := log.Apply(c, InteractionType("unknown"))
	if err == nil {
		t.Error("expected error for unknown interaction type")
	}
}

func TestInteractionLog_HistoryCap(t *testing.T) {
	c := makeTestCompanion()
	log := NewInteractionLog()
	log.Cooldown = 0

	for i := 0; i < 120; i++ {
		_, _ = log.Apply(c, InteractionRest)
	}
	if log.TotalInteractions() != 100 {
		t.Errorf("expected history capped at 100, got %d", log.TotalInteractions())
	}
}

func TestInteractionLog_Counts(t *testing.T) {
	c := makeTestCompanion()
	log := NewInteractionLog()
	log.Cooldown = 0

	_, _ = log.Apply(c, InteractionFeed)
	_, _ = log.Apply(c, InteractionFeed)
	_, _ = log.Apply(c, InteractionPlay)

	counts := log.InteractionCounts()
	if counts[InteractionFeed] != 2 {
		t.Errorf("expected 2 feeds, got %d", counts[InteractionFeed])
	}
	if counts[InteractionPlay] != 1 {
		t.Errorf("expected 1 play, got %d", counts[InteractionPlay])
	}
}

func TestInteractionLog_TimeSinceLast(t *testing.T) {
	log := NewInteractionLog()
	if log.TimeSinceLastInteraction() != 0 {
		t.Error("expected 0 for empty log")
	}
}

func TestDecayStats(t *testing.T) {
	c := makeTestCompanion()
	c.Soul.Energy = 0.8
	c.Soul.Mood = 0.8

	DecayStats(c, 5*time.Hour)

	if c.Soul.Energy >= 0.8 {
		t.Errorf("expected energy to decay, got %f", c.Soul.Energy)
	}
	if c.Soul.Mood >= 0.8 {
		t.Errorf("expected mood to decay, got %f", c.Soul.Mood)
	}
	// Should not go below 0.1.
	if c.Soul.Energy < 0.1 {
		t.Errorf("energy decayed below floor: %f", c.Soul.Energy)
	}
	if c.Soul.Mood < 0.1 {
		t.Errorf("mood decayed below floor: %f", c.Soul.Mood)
	}
}

func TestDecayStats_NoDecayForZero(t *testing.T) {
	c := makeTestCompanion()
	original := c.Soul.Energy
	DecayStats(c, 0)
	if c.Soul.Energy != original {
		t.Errorf("expected no change for 0 elapsed, got %f", c.Soul.Energy)
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		v, lo, hi, want float64
	}{
		{0.5, 0, 1, 0.5},
		{-1, 0, 1, 0},
		{2, 0, 1, 1},
		{0, 0, 0, 0},
	}
	for _, tt := range tests {
		got := clamp(tt.v, tt.lo, tt.hi)
		if got != tt.want {
			t.Errorf("clamp(%f, %f, %f) = %f, want %f", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}
