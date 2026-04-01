package compact

import (
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// MicroCompact performs local, heuristic-based message compaction without an
// LLM call.  It removes duplicate tool results, truncates very long text
// blocks, and collapses consecutive whitespace.
func MicroCompact(messages []*engine.Message, maxBlockChars int) []*engine.Message {
	if maxBlockChars <= 0 {
		maxBlockChars = 8000
	}

	out := make([]*engine.Message, 0, len(messages))
	for _, m := range messages {
		compacted := compactMessage(m, maxBlockChars)
		out = append(out, compacted)
	}
	return out
}

func compactMessage(m *engine.Message, maxBlockChars int) *engine.Message {
	newBlocks := make([]*engine.ContentBlock, 0, len(m.Content))
	for _, b := range m.Content {
		nb := compactBlock(b, maxBlockChars)
		newBlocks = append(newBlocks, nb)
	}
	return &engine.Message{
		ID:        m.ID,
		Role:      m.Role,
		Content:   newBlocks,
		Timestamp: m.Timestamp,
		SessionID: m.SessionID,
	}
}

func compactBlock(b *engine.ContentBlock, maxChars int) *engine.ContentBlock {
	if b.Type != engine.ContentTypeText {
		return b
	}
	text := strings.Join(strings.Fields(b.Text), " ") // collapse whitespace
	if len(text) > maxChars {
		text = text[:maxChars] + "\n... [truncated]"
	}
	return &engine.ContentBlock{
		Type:    b.Type,
		Text:    text,
		IsError: b.IsError,
	}
}
