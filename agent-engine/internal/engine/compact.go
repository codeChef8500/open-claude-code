package engine

import (
	"context"
	"fmt"
	"strings"
)

const compactSystemPrompt = `You are a conversation compactor. Given a conversation history, produce a concise 
summary that preserves all important context: decisions made, code written, problems solved, and any open tasks.
Start with "## Conversation Summary" and use bullet points. Be thorough but concise.`

// CompactMessages summarises the given messages into a single synthetic user
// message, then returns a fresh message slice suitable for resuming the session.
// This is the Go equivalent of /compact in the TypeScript implementation.
func CompactMessages(
	ctx context.Context,
	prov ModelCaller,
	messages []*Message,
	model string,
) ([]*Message, string, error) {

	if len(messages) == 0 {
		return messages, "", nil
	}

	// Build plain-text transcript for the summariser.
	var sb strings.Builder
	for _, m := range messages {
		role := string(m.Role)
		for _, b := range m.Content {
			switch b.Type {
			case ContentTypeText:
				if b.Text != "" {
					fmt.Fprintf(&sb, "[%s]: %s\n\n", role, b.Text)
				}
			case ContentTypeToolUse:
				fmt.Fprintf(&sb, "[%s uses tool %s]\n\n", role, b.ToolName)
			case ContentTypeToolResult:
				var parts []string
				for _, inner := range b.Content {
					if inner.Type == ContentTypeText {
						parts = append(parts, inner.Text)
					}
				}
				fmt.Fprintf(&sb, "[tool result]: %s\n\n", strings.Join(parts, " "))
			}
		}
	}

	summariserMsg := []*Message{
		{
			Role: RoleUser,
			Content: []*ContentBlock{
				{Type: ContentTypeText, Text: "Summarise this conversation:\n\n" + sb.String()},
			},
		},
	}

	params := CallParams{
		Model:        model,
		MaxTokens:    2048,
		SystemPrompt: compactSystemPrompt,
		Messages:     summariserMsg,
	}

	eventCh, err := prov.CallModel(ctx, params)
	if err != nil {
		return nil, "", fmt.Errorf("compact: %w", err)
	}

	var summary string
	for ev := range eventCh {
		if ev.Type == EventTextDelta {
			summary += ev.Text
		}
		if ev.Type == EventError {
			return nil, "", fmt.Errorf("compact provider error: %s", ev.Error)
		}
	}

	// Build the compacted message history: one synthetic context message.
	compactedMessages := []*Message{
		{
			Role: RoleUser,
			Content: []*ContentBlock{
				{Type: ContentTypeText, Text: "[Previous conversation summary]\n\n" + summary},
			},
		},
		{
			Role: RoleAssistant,
			Content: []*ContentBlock{
				{Type: ContentTypeText, Text: "I have reviewed the conversation summary and am ready to continue."},
			},
		},
	}

	return compactedMessages, summary, nil
}

// EstimateTokens provides a rough token estimate (4 chars ≈ 1 token).
func EstimateTokens(messages []*Message) int {
	total := 0
	for _, m := range messages {
		for _, b := range m.Content {
			total += len(b.Text)/4 + len(b.Thinking)/4
		}
	}
	return total
}
