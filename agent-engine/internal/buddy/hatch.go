package buddy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

const hatchSystemPrompt = `You are a companion naming oracle. Given a companion's species and rarity, 
suggest a single creative, memorable name. Respond with JSON only: {"name": "..."}`

// Hatch uses an LLM side-query to give the companion a generated name, then
// returns the fully assembled Companion.
func Hatch(ctx context.Context, prov provider.Provider, seed uint32) (*Companion, error) {
	bones := GenerateBones(seed)

	name, err := generateName(ctx, prov, bones)
	if err != nil {
		// Fall back to a deterministic name if the LLM call fails.
		name = defaultName(bones)
	}

	return &Companion{
		Bones: bones,
		Soul: CompanionSoul{
			Name:      name,
			Mood:      0.7,
			Energy:    0.8,
			Affection: 0.5,
		},
	}, nil
}

// HatchWithSeed is like Hatch but skips the LLM call and uses a default name.
func HatchWithSeed(seed uint32) *Companion {
	bones := GenerateBones(seed)
	return &Companion{
		Bones: bones,
		Soul: CompanionSoul{
			Name:      defaultName(bones),
			Mood:      0.7,
			Energy:    0.8,
			Affection: 0.5,
		},
	}
}

func generateName(ctx context.Context, prov provider.Provider, bones CompanionBones) (string, error) {
	userText := fmt.Sprintf("Species: %s\nRarity: %s\nPrimary hue: %.0f°\n\nSuggest one name.",
		bones.Species, bones.Rarity, bones.PrimaryHue)

	params := provider.CallParams{
		Model:        "claude-haiku-4-5",
		MaxTokens:    64,
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
		return "", err
	}

	var sb strings.Builder
	for ev := range eventCh {
		if ev.Type == engine.EventTextDelta {
			sb.WriteString(ev.Text)
		}
	}

	var resp struct {
		Name string `json:"name"`
	}
	text := strings.TrimSpace(sb.String())
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &resp); err != nil {
		return "", fmt.Errorf("name parse: %w", err)
	}
	if resp.Name == "" {
		return "", fmt.Errorf("empty name from LLM")
	}
	return resp.Name, nil
}

// defaultName builds a deterministic fallback name from bones.
func defaultName(b CompanionBones) string {
	rng := newMulberry32(uint32(b.PrimaryHue*1000) ^ uint32(len(b.Species)))
	prefixes := []string{"Astra", "Blaze", "Cosmo", "Dusk", "Echo",
		"Flux", "Glim", "Haze", "Iris", "Jest",
		"Koda", "Lune", "Myst", "Nova", "Onyx"}
	idx := int(rng.next()) % len(prefixes)
	s := string(b.Species)
	suffix := capitalise(s[:1]) + s[1:3]
	return prefixes[idx] + suffix
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
