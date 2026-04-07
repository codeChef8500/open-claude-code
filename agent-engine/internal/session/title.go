package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

// ────────────────────────────────────────────────────────────────────────────
// Session title generation — aligned with claude-code-main
// src/utils/sessionTitle.ts
// ────────────────────────────────────────────────────────────────────────────

const (
	// TitleModel is the preferred model for title generation (cheap & fast).
	TitleModel = "claude-3-haiku-20240307"

	titleSystemPrompt = `You are a concise title generator. Given a conversation, generate a very short title (max 60 characters) that captures the main topic. Return ONLY a JSON object: {"title": "..."}`

	// MaxTitleLength is the maximum length of a generated title.
	MaxTitleLength = 60

	// MaxConversationCharsForTitle limits how much conversation text is sent
	// to the title generator.
	MaxConversationCharsForTitle = 4000
)

// GenerateSessionTitle generates a concise title for a session from its messages.
// Uses a lightweight LLM call (Haiku).
func GenerateSessionTitle(
	ctx context.Context,
	prov provider.Provider,
	messages []*engine.Message,
) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Build a condensed conversation excerpt
	excerpt := buildTitleExcerpt(messages)
	if excerpt == "" {
		return "", nil
	}

	params := provider.CallParams{
		Model:        TitleModel,
		MaxTokens:    128,
		SystemPrompt: titleSystemPrompt,
		Messages: []*engine.Message{
			{
				Role: engine.RoleUser,
				Content: []*engine.ContentBlock{{
					Type: engine.ContentTypeText,
					Text: "Generate a title for this conversation:\n\n" + excerpt,
				}},
			},
		},
		UsePromptCache: false,
	}

	eventCh, err := prov.CallModel(ctx, params)
	if err != nil {
		return "", fmt.Errorf("title generation: %w", err)
	}

	var raw strings.Builder
	for ev := range eventCh {
		if ev.Type == engine.EventTextDelta {
			raw.WriteString(ev.Text)
		}
	}

	title := parseTitleResponse(raw.String())
	if title == "" {
		slog.Debug("title generation: empty or unparseable response",
			slog.String("raw", raw.String()))
		return fallbackTitle(messages), nil
	}

	return title, nil
}

// buildTitleExcerpt builds a concise excerpt of the conversation for title generation.
func buildTitleExcerpt(messages []*engine.Message) string {
	var sb strings.Builder
	for _, m := range messages {
		for _, b := range m.Content {
			if b.Type == engine.ContentTypeText && b.Text != "" {
				role := string(m.Role)
				text := b.Text
				// Truncate very long messages
				if len(text) > 500 {
					text = text[:500] + "..."
				}
				fmt.Fprintf(&sb, "[%s]: %s\n", role, text)
				if sb.Len() > MaxConversationCharsForTitle {
					return sb.String()
				}
			}
		}
	}
	return sb.String()
}

// parseTitleResponse extracts the title from the LLM JSON response.
func parseTitleResponse(raw string) string {
	raw = strings.TrimSpace(raw)

	// Try to find JSON in the response
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		jsonStr := raw[start : end+1]
		var resp struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &resp); err == nil && resp.Title != "" {
			title := strings.TrimSpace(resp.Title)
			if len(title) > MaxTitleLength {
				title = title[:MaxTitleLength-3] + "..."
			}
			return title
		}
	}

	// If not JSON, try to use the raw text directly (if short enough)
	if len(raw) > 0 && len(raw) <= MaxTitleLength+20 {
		// Strip quotes
		raw = strings.Trim(raw, `"'`)
		if len(raw) > MaxTitleLength {
			raw = raw[:MaxTitleLength-3] + "..."
		}
		return raw
	}

	return ""
}

// fallbackTitle generates a simple title from the first user message.
func fallbackTitle(messages []*engine.Message) string {
	for _, m := range messages {
		if m.Role != engine.RoleUser {
			continue
		}
		for _, b := range m.Content {
			if b.Type == engine.ContentTypeText && b.Text != "" {
				text := strings.TrimSpace(b.Text)
				if len(text) > MaxTitleLength-3 {
					text = text[:MaxTitleLength-3] + "..."
				}
				return text
			}
		}
	}
	return "Untitled session"
}
