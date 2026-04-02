package compact

import (
	"github.com/wall-ai/agent-engine/internal/engine"
)

// SnipOptions controls which messages the snip pass removes.
type SnipOptions struct {
	// KeepFirstN preserves the first N messages unconditionally (they
	// typically contain key instructions).  Default: 2.
	KeepFirstN int
	// KeepLastN preserves the last N messages unconditionally (recent context
	// is most relevant).  Default: 6.
	KeepLastN int
	// RemoveThinking strips <thinking> blocks from all messages.
	RemoveThinking bool
	// MaxMessagesToKeep is the hard ceiling on the total message count after
	// snipping.  0 means no ceiling.
	MaxMessagesToKeep int
}

// Snip removes less-valuable messages from the middle of the history to
// reduce token usage without an LLM call.  It always keeps the first
// KeepFirstN and last KeepLastN messages.  Surplus messages in the middle
// are dropped in reverse-chronological order (oldest first).
func Snip(messages []*engine.Message, opts SnipOptions) []*engine.Message {
	keepFirst := opts.KeepFirstN
	if keepFirst <= 0 {
		keepFirst = 2
	}
	keepLast := opts.KeepLastN
	if keepLast <= 0 {
		keepLast = 6
	}

	n := len(messages)
	if n == 0 {
		return messages
	}

	// Strip thinking blocks if requested.
	if opts.RemoveThinking {
		messages = stripThinkingBlocks(messages)
	}

	// If already within limits, just enforce MaxMessagesToKeep.
	if opts.MaxMessagesToKeep > 0 && n > opts.MaxMessagesToKeep {
		// Keep first keepFirst + last keepLast, fill remaining budget from the end.
		budget := opts.MaxMessagesToKeep
		head := keepFirst
		if head > budget {
			head = budget
		}
		tail := budget - head
		if tail > keepLast {
			tail = keepLast
		}
		if tail < 0 {
			tail = 0
		}
		result := make([]*engine.Message, 0, budget)
		result = append(result, messages[:head]...)
		if n-tail > head {
			result = append(result, messages[n-tail:]...)
		}
		return result
	}

	// If there is no surplus, return as-is.
	if n <= keepFirst+keepLast {
		return messages
	}

	head := messages[:keepFirst]
	tail := messages[n-keepLast:]

	result := make([]*engine.Message, 0, keepFirst+keepLast)
	result = append(result, head...)
	result = append(result, tail...)
	return result
}

// stripThinkingBlocks removes ContentTypeThinking blocks from all messages.
func stripThinkingBlocks(messages []*engine.Message) []*engine.Message {
	out := make([]*engine.Message, 0, len(messages))
	for _, m := range messages {
		var newContent []*engine.ContentBlock
		for _, b := range m.Content {
			if b.Type == engine.ContentTypeThinking {
				continue
			}
			newContent = append(newContent, b)
		}
		if len(newContent) == 0 && len(m.Content) > 0 {
			// Don't emit a fully-empty message.
			continue
		}
		out = append(out, &engine.Message{
			ID:        m.ID,
			Role:      m.Role,
			Content:   newContent,
			Timestamp: m.Timestamp,
			SessionID: m.SessionID,
		})
	}
	return out
}
