package service

import (
	"strings"
	"unicode/utf8"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// EstimateTokens returns a rough token count for a text string.
// Uses the heuristic: 1 token ≈ 4 UTF-8 characters (works well for English/code).
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	chars := utf8.RuneCountInString(text)
	tokens := chars / 4
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}

// EstimateMessagesTokens sums EstimateTokens across all text blocks in a
// message slice, adding 4 tokens per message for role/structure overhead.
func EstimateMessagesTokens(messages []*engine.Message) int {
	total := 0
	for _, m := range messages {
		total += 4 // role overhead
		for _, b := range m.Content {
			switch b.Type {
			case engine.ContentTypeText:
				total += EstimateTokens(b.Text)
			case engine.ContentTypeThinking:
				total += EstimateTokens(b.Thinking)
			case engine.ContentTypeToolResult:
				for _, c := range b.Content {
					if c.Type == engine.ContentTypeText {
						total += EstimateTokens(c.Text)
					}
				}
			}
		}
	}
	return total
}

// ExceedsThreshold reports whether the estimated token count exceeds
// fraction * maxTokens.
func ExceedsThreshold(messages []*engine.Message, maxTokens int, fraction float64) bool {
	if maxTokens <= 0 {
		return false
	}
	used := EstimateMessagesTokens(messages)
	return float64(used) >= float64(maxTokens)*fraction
}

// TruncateToTokenBudget removes the oldest messages until the estimated token
// count is below budget, always keeping system/first messages if possible.
func TruncateToTokenBudget(messages []*engine.Message, budget int) []*engine.Message {
	for len(messages) > 1 && EstimateMessagesTokens(messages) > budget {
		// Remove oldest non-system message (index 0 is usually the earliest user msg)
		messages = messages[1:]
	}
	return messages
}

// CountWords is a quick word-count utility used for display purposes.
func CountWords(text string) int {
	return len(strings.Fields(text))
}
