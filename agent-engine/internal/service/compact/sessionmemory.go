package compact

import (
	"context"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

const sessionMemorySystemPrompt = `You are a session memory extractor.
Given a conversation, extract the most important durable facts that should be remembered
across future sessions: file paths modified, decisions made, coding conventions observed,
user preferences stated, and any open tasks or blockers.

Return ONLY a bulleted list (one fact per line, starting with "- ").
Be concise. Omit anything transient or easily re-discovered.`

// SessionMemory holds the extracted durable facts from a session.
type SessionMemory struct {
	Facts []string
}

// String renders the session memory as a bulleted markdown list.
func (s *SessionMemory) String() string {
	if len(s.Facts) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, f := range s.Facts {
		sb.WriteString("- ")
		sb.WriteString(strings.TrimPrefix(strings.TrimSpace(f), "- "))
		sb.WriteString("\n")
	}
	return sb.String()
}

// ExtractSessionMemory asks the LLM to extract key facts from the conversation
// and returns a SessionMemory.  On error it returns a non-nil memory with an
// empty facts slice so callers don't need to nil-check.
func ExtractSessionMemory(
	ctx context.Context,
	prov provider.Provider,
	messages []*engine.Message,
	model string,
) (*SessionMemory, error) {
	if len(messages) == 0 {
		return &SessionMemory{}, nil
	}

	// Build a condensed transcript.
	var sb strings.Builder
	for _, m := range messages {
		for _, b := range m.Content {
			if b.Type == engine.ContentTypeText && b.Text != "" {
				fmt.Fprintf(&sb, "[%s]: %s\n\n", m.Role, b.Text)
			}
		}
	}

	params := provider.CallParams{
		Model:        model,
		MaxTokens:    1024,
		SystemPrompt: sessionMemorySystemPrompt,
		Messages: []*engine.Message{
			{
				Role: engine.RoleUser,
				Content: []*engine.ContentBlock{{
					Type: engine.ContentTypeText,
					Text: "Extract session memory from:\n\n" + sb.String(),
				}},
			},
		},
		UsePromptCache: false,
	}

	eventCh, err := prov.CallModel(ctx, params)
	if err != nil {
		return &SessionMemory{}, fmt.Errorf("session memory: %w", err)
	}

	var raw strings.Builder
	for ev := range eventCh {
		if ev.Type == engine.EventTextDelta {
			raw.WriteString(ev.Text)
		}
	}

	facts := parseBulletList(strings.TrimSpace(raw.String()))
	return &SessionMemory{Facts: facts}, nil
}

// parseBulletList splits a bulleted list into individual items, stripping the
// leading "- " or "* " marker.
func parseBulletList(text string) []string {
	var facts []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Accept "- ", "* ", or numbered "1. " prefixes.
		for _, prefix := range []string{"- ", "* ", "• "} {
			if strings.HasPrefix(line, prefix) {
				line = strings.TrimPrefix(line, prefix)
				break
			}
		}
		if line != "" {
			facts = append(facts, line)
		}
	}
	return facts
}
