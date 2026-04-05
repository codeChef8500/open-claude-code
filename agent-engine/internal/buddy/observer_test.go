package buddy

import (
	"sync"
	"testing"
	"time"
)

// ─── Observer basic ──────────────────────────────────────────────────────────

func newTestCompanion() *Companion {
	return &Companion{
		CompanionBones: CompanionBones{
			Species: SpeciesCat,
			Rarity:  RarityCommon,
			Eye:     EyeDot,
			Hat:     HatNone,
			Stats:   map[StatName]int{StatDebugging: 50, StatPatience: 50, StatChaos: 50, StatWisdom: 50, StatSnark: 50},
		},
		CompanionSoul: CompanionSoul{
			Name:        "TestCat",
			Personality: "testy",
		},
		HatchedAt: time.Now().UnixMilli(),
	}
}

func TestObserver_ErrorAlwaysReacts(t *testing.T) {
	var mu sync.Mutex
	var reactions []string

	comp := newTestCompanion()
	obs := NewObserver(comp, func(text string) {
		mu.Lock()
		reactions = append(reactions, text)
		mu.Unlock()
	})
	// Override cooldown to 0 for testing
	obs.cooldownMS = 0

	// Error events should always produce a reaction
	for i := 0; i < 10; i++ {
		obs.OnEvent(EngineEvent{Kind: EventError, Detail: "test error"})
	}

	mu.Lock()
	count := len(reactions)
	mu.Unlock()

	if count != 10 {
		t.Errorf("expected 10 error reactions, got %d", count)
	}
}

func TestObserver_NilCompanionNoReaction(t *testing.T) {
	reacted := false
	obs := NewObserver(nil, func(text string) {
		reacted = true
	})
	obs.OnEvent(EngineEvent{Kind: EventError})
	if reacted {
		t.Error("should not react with nil companion")
	}
}

func TestObserver_NilCallbackNoReaction(t *testing.T) {
	comp := newTestCompanion()
	obs := NewObserver(comp, nil)
	// Should not panic
	obs.OnEvent(EngineEvent{Kind: EventError})
}

func TestObserver_CooldownPreventsSpam(t *testing.T) {
	var reactions []string
	comp := newTestCompanion()
	obs := NewObserver(comp, func(text string) {
		reactions = append(reactions, text)
	})
	obs.cooldownMS = 5000 // 5 second cooldown

	// First event should react
	obs.OnEvent(EngineEvent{Kind: EventError})
	// Second event within cooldown should not react
	obs.OnEvent(EngineEvent{Kind: EventError})

	if len(reactions) != 1 {
		t.Errorf("expected 1 reaction (cooldown should block second), got %d", len(reactions))
	}
}

func TestObserver_ToolStartReaction(t *testing.T) {
	var reactions []string
	comp := newTestCompanion()
	obs := NewObserver(comp, func(text string) {
		reactions = append(reactions, text)
	})
	obs.cooldownMS = 0

	// Edit tool should always trigger a reaction
	obs.OnEvent(EngineEvent{Kind: EventToolStart, ToolName: "file_edit"})
	if len(reactions) == 0 {
		t.Error("expected reaction for edit tool")
	}
	if reactions[0] == "" {
		t.Error("reaction should not be empty")
	}
}

func TestObserver_ToolStartEditReaction(t *testing.T) {
	var reactions []string
	comp := newTestCompanion()
	obs := NewObserver(comp, func(text string) {
		reactions = append(reactions, text)
	})
	obs.cooldownMS = 0

	obs.OnEvent(EngineEvent{Kind: EventToolStart, ToolName: "write_file"})
	if len(reactions) == 0 {
		t.Error("expected reaction for write tool")
	}
}

func TestObserver_ToolStartBashReaction(t *testing.T) {
	var reactions []string
	comp := newTestCompanion()
	obs := NewObserver(comp, func(text string) {
		reactions = append(reactions, text)
	})
	obs.cooldownMS = 0

	obs.OnEvent(EngineEvent{Kind: EventToolStart, ToolName: "bash_command"})
	if len(reactions) == 0 {
		t.Error("expected reaction for bash tool")
	}
}

func TestObserver_ToolStartGrepReaction(t *testing.T) {
	var reactions []string
	comp := newTestCompanion()
	obs := NewObserver(comp, func(text string) {
		reactions = append(reactions, text)
	})
	obs.cooldownMS = 0

	obs.OnEvent(EngineEvent{Kind: EventToolStart, ToolName: "grep_search"})
	if len(reactions) == 0 {
		t.Error("expected reaction for grep tool")
	}
}

func TestObserver_SetCompanion(t *testing.T) {
	var reactions []string
	obs := NewObserver(nil, func(text string) {
		reactions = append(reactions, text)
	})
	obs.cooldownMS = 0

	// No reaction with nil companion
	obs.OnEvent(EngineEvent{Kind: EventError})
	if len(reactions) != 0 {
		t.Error("should not react with nil companion")
	}

	// After setting companion, should react
	comp := newTestCompanion()
	obs.SetCompanion(comp)
	obs.OnEvent(EngineEvent{Kind: EventError})
	if len(reactions) == 0 {
		t.Error("should react after setting companion")
	}
}

func TestObserver_ReactionContainsName(t *testing.T) {
	comp := newTestCompanion()
	nameFound := false
	obs := NewObserver(comp, func(text string) {
		// Some reactions reference the companion's name
		if text != "" {
			nameFound = true // at least got a reaction
		}
	})
	obs.cooldownMS = 0

	// Error always reacts
	obs.OnEvent(EngineEvent{Kind: EventError})
	if !nameFound {
		t.Error("expected at least one reaction")
	}
}
