package prompt

import (
	"github.com/wall-ai/agent-engine/internal/engine"
)

// TrimThinkingBlocks removes standalone thinking blocks from messages before
// sending to the API (they must be paired with their text response blocks).
func TrimThinkingBlocks(messages []*engine.Message) []*engine.Message {
	result := make([]*engine.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role != engine.RoleAssistant {
			result = append(result, m)
			continue
		}
		// Keep only non-thinking content unless paired with tool_use or text.
		var filtered []*engine.ContentBlock
		for _, b := range m.Content {
			if b.Type != engine.ContentTypeThinking {
				filtered = append(filtered, b)
			}
		}
		if len(filtered) > 0 {
			cp := *m
			cp.Content = filtered
			result = append(result, &cp)
		}
	}
	return result
}

// TruncateToolResults caps all tool result text blocks to maxChars characters.
func TruncateToolResults(messages []*engine.Message, maxChars int) []*engine.Message {
	if maxChars <= 0 {
		return messages
	}
	result := make([]*engine.Message, len(messages))
	for i, m := range messages {
		result[i] = truncateMessageResults(m, maxChars)
	}
	return result
}

func truncateMessageResults(m *engine.Message, maxChars int) *engine.Message {
	if m.Role != engine.RoleUser {
		return m
	}
	changed := false
	newContent := make([]*engine.ContentBlock, len(m.Content))
	for i, b := range m.Content {
		if b.Type != engine.ContentTypeToolResult {
			newContent[i] = b
			continue
		}
		newInner := make([]*engine.ContentBlock, len(b.Content))
		for j, inner := range b.Content {
			if inner.Type == engine.ContentTypeText && len(inner.Text) > maxChars {
				cp := *inner
				cp.Text = inner.Text[:maxChars] + "\n[... truncated ...]"
				newInner[j] = &cp
				changed = true
			} else {
				newInner[j] = inner
			}
		}
		cp := *b
		cp.Content = newInner
		newContent[i] = &cp
	}
	if !changed {
		return m
	}
	cp := *m
	cp.Content = newContent
	return &cp
}

// PrepareMessages applies all serialisation transforms before an API call.
func PrepareMessages(messages []*engine.Message, maxToolResultChars int) []*engine.Message {
	messages = TruncateToolResults(messages, maxToolResultChars)
	return messages
}
