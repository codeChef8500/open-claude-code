package prompt

import "github.com/wall-ai/agent-engine/internal/engine"

// EnforceThinkingIntegrity ensures that thinking blocks always appear before
// their paired text/tool_use blocks in assistant messages, and removes orphaned
// thinking blocks. This matches the Anthropic API requirement.
func EnforceThinkingIntegrity(messages []*engine.Message) []*engine.Message {
	result := make([]*engine.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role != engine.RoleAssistant {
			result = append(result, m)
			continue
		}
		result = append(result, reorderThinkingBlocks(m))
	}
	return result
}

func reorderThinkingBlocks(m *engine.Message) *engine.Message {
	var thinking []*engine.ContentBlock
	var rest []*engine.ContentBlock

	for _, b := range m.Content {
		if b.Type == engine.ContentTypeThinking {
			thinking = append(thinking, b)
		} else {
			rest = append(rest, b)
		}
	}

	if len(thinking) == 0 {
		return m
	}

	// If no non-thinking content, drop the orphaned thinking blocks.
	if len(rest) == 0 {
		cp := *m
		cp.Content = nil
		return &cp
	}

	// Reconstruct: thinking blocks first, then rest.
	combined := append(thinking, rest...)
	cp := *m
	cp.Content = combined
	return &cp
}

// HasThinkingBlocks reports whether any message contains thinking content.
func HasThinkingBlocks(messages []*engine.Message) bool {
	for _, m := range messages {
		for _, b := range m.Content {
			if b.Type == engine.ContentTypeThinking {
				return true
			}
		}
	}
	return false
}
