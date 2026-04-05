package buddy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

const hatchSystemPrompt = `You are a companion soul oracle. Given a companion's species, rarity, and an inspiration seed number, generate a creative name and a short personality description (one sentence).

Respond with JSON only: {"name": "...", "personality": "..."}`

// Hatch uses an LLM side-query to generate the companion's soul (name + personality),
// then returns the fully assembled Companion.
// Matches claude-code-main's hatching flow: roll bones → LLM generates soul → merge.
func Hatch(ctx context.Context, prov provider.Provider, userID string) (*Companion, error) {
	r := RollCompanion(userID)

	soul, err := generateSoul(ctx, prov, r.Bones, r.InspirationSeed)
	if err != nil {
		// Fall back to deterministic soul if the LLM call fails.
		soul = defaultSoul(r.Bones, r.InspirationSeed)
	}

	return &Companion{
		CompanionBones: r.Bones,
		CompanionSoul:  *soul,
		HatchedAt:      time.Now().UnixMilli(),
	}, nil
}

// HatchWithoutLLM hatches a companion without LLM (test/offline use).
func HatchWithoutLLM(userID string) *Companion {
	r := RollCompanion(userID)
	soul := defaultSoul(r.Bones, r.InspirationSeed)
	return &Companion{
		CompanionBones: r.Bones,
		CompanionSoul:  *soul,
		HatchedAt:      time.Now().UnixMilli(),
	}
}

func generateSoul(ctx context.Context, prov provider.Provider, bones CompanionBones, inspirationSeed int) (*CompanionSoul, error) {
	userText := fmt.Sprintf(
		"Species: %s\nRarity: %s\nShiny: %v\nInspiration seed: %d\n\nGenerate a name and personality.",
		bones.Species, bones.Rarity, bones.Shiny, inspirationSeed,
	)

	params := provider.CallParams{
		Model:        "claude-haiku-4-5",
		MaxTokens:    128,
		SystemPrompt: hatchSystemPrompt,
		Messages: []*engine.Message{
			{
				Role:    engine.RoleUser,
				Content: []*engine.ContentBlock{{Type: engine.ContentTypeText, Text: userText}},
			},
		},
	}

	eventCh, err := prov.CallModel(ctx, params)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	for ev := range eventCh {
		if ev.Type == engine.EventTextDelta {
			sb.WriteString(ev.Text)
		}
	}

	var resp struct {
		Name        string `json:"name"`
		Personality string `json:"personality"`
	}
	text := strings.TrimSpace(sb.String())
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &resp); err != nil {
		return nil, fmt.Errorf("soul parse: %w", err)
	}
	if resp.Name == "" {
		return nil, fmt.Errorf("empty name from LLM")
	}
	return &CompanionSoul{
		Name:        resp.Name,
		Personality: resp.Personality,
	}, nil
}

// defaultSoul builds a deterministic fallback soul from bones.
func defaultSoul(b CompanionBones, inspirationSeed int) *CompanionSoul {
	rng := newMulberry32(uint32(inspirationSeed))
	prefixes := []string{
		"Astra", "Blaze", "Cosmo", "Dusk", "Echo",
		"Flux", "Glim", "Haze", "Iris", "Jest",
		"Koda", "Lune", "Myst", "Nova", "Onyx",
	}
	idx := int(rng.next()) % len(prefixes)
	s := string(b.Species)
	suffix := capitalise(s[:1])
	if len(s) >= 3 {
		suffix += s[1:3]
	}
	personalities := []string{
		"calm and methodical, loves tracing bugs",
		"chaotic but endearing, easily distracted by shiny things",
		"wise beyond their years, always offering sage advice",
		"mischievous and playful, hides easter eggs everywhere",
		"stoic and reliable, the quiet anchor of any session",
	}
	pIdx := int(rng.next()) % len(personalities)
	return &CompanionSoul{
		Name:        prefixes[idx] + suffix,
		Personality: personalities[pIdx],
	}
}

// capitalise upper-cases the first byte of a pure-ASCII string.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	b := s[0]
	if b >= 'a' && b <= 'z' {
		b -= 32
	}
	return string(b) + s[1:]
}
