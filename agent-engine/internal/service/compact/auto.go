package compact

import (
	"context"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

const (
	// CompactThreshold is the fraction of maxTokens at which auto-compact triggers.
	CompactThreshold = 0.80

	autoCompactSystemPrompt = `You are a conversation summariser. 
Produce a concise but complete summary of the conversation so far, preserving all key decisions, 
file paths, code changes, and outstanding tasks. The summary will replace the full history 
to free up context window space.
Return plain text only — no JSON, no markdown headers.`
)

// AutoCompactResult holds the output of an LLM-driven compact operation.
type AutoCompactResult struct {
	Summary      string
	TokensBefore int
	TokensAfter  int
}

// RunAutoCompact sends the conversation history to the LLM for summarisation
// and returns a compact result.  The caller should replace the message history
// with a single synthetic assistant message containing the summary.
func RunAutoCompact(
	ctx context.Context,
	prov provider.Provider,
	messages []*engine.Message,
	model string,
) (*AutoCompactResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("auto compact: no messages to compact")
	}

	// Build a plain-text transcript for the summariser.
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
		MaxTokens:    4096,
		SystemPrompt: autoCompactSystemPrompt,
		Messages: []*engine.Message{
			{
				Role: engine.RoleUser,
				Content: []*engine.ContentBlock{{
					Type: engine.ContentTypeText,
					Text: "Summarise this conversation:\n\n" + sb.String(),
				}},
			},
		},
		UsePromptCache: false,
	}

	eventCh, err := prov.CallModel(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("auto compact: %w", err)
	}

	var summary strings.Builder
	for ev := range eventCh {
		if ev.Type == engine.EventTextDelta {
			summary.WriteString(ev.Text)
		}
	}

	return &AutoCompactResult{
		Summary:      strings.TrimSpace(summary.String()),
		TokensBefore: estimateTokens(sb.String()),
		TokensAfter:  estimateTokens(summary.String()),
	}, nil
}

// estimateTokens gives a rough character-based token estimate (1 token ≈ 4 chars).
func estimateTokens(text string) int {
	return len(text) / 4
}
