package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
	"time"
)

const extractorSystemPrompt = `You are a memory extraction assistant. Given a conversation, extract concise, 
reusable facts or preferences the user has expressed. Return them as a JSON array of strings.
Each item should be a single, standalone fact. Only include facts worth remembering long-term.
Return ONLY a JSON array, no other text.`

// ExtractMemories uses a side-query LLM call to distil key facts from messages.
func ExtractMemories(
	ctx context.Context,
	prov provider.Provider,
	messages []*engine.Message,
	sessionID string,
) ([]*ExtractedMemory, error) {

	if len(messages) == 0 {
		return nil, nil
	}

	// Build a plain-text transcript for the extractor.
	var sb strings.Builder
	for _, m := range messages {
		role := string(m.Role)
		for _, b := range m.Content {
			if b.Type == engine.ContentTypeText && b.Text != "" {
				fmt.Fprintf(&sb, "[%s]: %s\n", role, b.Text)
			}
		}
	}

	userMsg := "Extract memorable facts from this conversation:\n\n" + sb.String()

	params := provider.CallParams{
		Model:          "claude-haiku-4-5",
		MaxTokens:      512,
		SystemPrompt:   extractorSystemPrompt,
		Messages: []*engine.Message{
			{
				Role:    engine.RoleUser,
				Content: []*engine.ContentBlock{{Type: engine.ContentTypeText, Text: userMsg}},
			},
		},
		UsePromptCache: true,
	}

	eventCh, err := prov.CallModel(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("memory extractor: %w", err)
	}

	var responseText string
	for ev := range eventCh {
		if ev.Type == engine.EventTextDelta {
			responseText += ev.Text
		}
	}

	facts := parseFactsJSON(responseText)
	now := time.Now()
	var memories []*ExtractedMemory
	for _, f := range facts {
		if f == "" {
			continue
		}
		memories = append(memories, &ExtractedMemory{
			ID:          uuid.New().String(),
			Content:     f,
			SessionID:   sessionID,
			ExtractedAt: now,
		})
	}
	return memories, nil
}

// parseFactsJSON parses a JSON array of strings from the extractor response.
func parseFactsJSON(text string) []string {
	text = strings.TrimSpace(text)
	// Strip markdown fences if present
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	if !strings.HasPrefix(text, "[") {
		return nil
	}

	// Simple manual JSON array parse to avoid dependencies.
	var facts []string
	text = strings.TrimPrefix(text, "[")
	text = strings.TrimSuffix(text, "]")
	for _, part := range strings.Split(text, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"`)
		if part != "" {
			facts = append(facts, part)
		}
	}
	return facts
}
