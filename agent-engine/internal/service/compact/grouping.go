package compact

import (
	"github.com/wall-ai/agent-engine/internal/engine"
)

// TurnGroup is a logical conversation turn consisting of an assistant message
// that may contain tool_use blocks and the subsequent user message(s) that
// carry the corresponding tool_result blocks.
type TurnGroup struct {
	// AssistantIdx is the index of the assistant message in the original slice.
	AssistantIdx int
	// ResultIdxs are the indices of the user messages that carry tool results
	// for the tool_use blocks in the assistant message.
	ResultIdxs []int
}

// GroupByTurns partitions a message slice into logical turns.
// A turn begins with each assistant message.  Immediately following user
// messages that contain only tool_result blocks are attached to that turn.
// Any other user messages start a new (user-only) entry with AssistantIdx=-1.
func GroupByTurns(messages []*engine.Message) []TurnGroup {
	var groups []TurnGroup
	i := 0
	for i < len(messages) {
		m := messages[i]
		if m.Role != engine.RoleAssistant {
			// Standalone user message.
			groups = append(groups, TurnGroup{AssistantIdx: i, ResultIdxs: nil})
			i++
			continue
		}

		g := TurnGroup{AssistantIdx: i}
		i++

		// Collect immediately following user messages that are pure tool-result carriers.
		for i < len(messages) && messages[i].Role == engine.RoleUser && isToolResultOnly(messages[i]) {
			g.ResultIdxs = append(g.ResultIdxs, i)
			i++
		}
		groups = append(groups, g)
	}
	return groups
}

// isToolResultOnly reports whether a message consists entirely of
// tool_result content blocks (no user text).
func isToolResultOnly(m *engine.Message) bool {
	if len(m.Content) == 0 {
		return false
	}
	for _, b := range m.Content {
		if b.Type != engine.ContentTypeToolResult {
			return false
		}
	}
	return true
}

// APIRound represents a single API round-trip: starting with either a user
// message or an assistant message, plus any following tool-result user messages.
// Aligned with claude-code-main groupMessagesByApiRound.
type APIRound struct {
	Messages []*engine.Message
}

// GroupMessagesByAPIRound groups messages into API rounds.  Each round begins
// with a user or assistant message.  Immediately following user messages that
// are pure tool-result carriers are attached to the preceding assistant round.
func GroupMessagesByAPIRound(messages []*engine.Message) []APIRound {
	if len(messages) == 0 {
		return nil
	}

	var rounds []APIRound
	var current [](*engine.Message)

	for _, m := range messages {
		if m.Role == engine.RoleAssistant {
			// Start a new round at each assistant message.
			if len(current) > 0 {
				rounds = append(rounds, APIRound{Messages: current})
			}
			current = []*engine.Message{m}
		} else if m.Role == engine.RoleUser {
			if len(current) > 0 && current[0].Role == engine.RoleAssistant && isToolResultOnly(m) {
				// Attach tool-result user messages to the preceding assistant round.
				current = append(current, m)
			} else {
				// User message that is not a tool result starts its own group.
				if len(current) > 0 {
					rounds = append(rounds, APIRound{Messages: current})
				}
				current = []*engine.Message{m}
			}
		} else {
			// System or other messages attach to current group.
			current = append(current, m)
		}
	}
	if len(current) > 0 {
		rounds = append(rounds, APIRound{Messages: current})
	}
	return rounds
}

// TruncateHeadForPTLRetry drops the oldest API-round groups from messages
// until tokenGap is covered. This is the escape hatch for when the compact
// request itself hits prompt-too-long. Returns nil when nothing can be dropped.
func TruncateHeadForPTLRetry(messages []*engine.Message, tokenGap int) []*engine.Message {
	groups := GroupMessagesByAPIRound(messages)
	if len(groups) < 2 {
		return nil
	}

	var dropCount int
	if tokenGap > 0 {
		acc := 0
		for _, g := range groups {
			acc += estimateTokensFromMessages(g.Messages)
			dropCount++
			if acc >= tokenGap {
				break
			}
		}
	} else {
		// Fallback: drop 20% of groups.
		dropCount = len(groups) / 5
		if dropCount < 1 {
			dropCount = 1
		}
	}

	// Keep at least one group.
	if dropCount >= len(groups) {
		dropCount = len(groups) - 1
	}
	if dropCount < 1 {
		return nil
	}

	// Flatten remaining groups.
	var result []*engine.Message
	for _, g := range groups[dropCount:] {
		result = append(result, g.Messages...)
	}

	// If the first message is assistant, prepend a synthetic user marker
	// so the API doesn't reject the sequence.
	if len(result) > 0 && result[0].Role == engine.RoleAssistant {
		marker := &engine.Message{
			Role: engine.RoleUser,
			Content: []*engine.ContentBlock{{
				Type: engine.ContentTypeText,
				Text: "[earlier conversation truncated for compaction retry]",
			}},
		}
		result = append([]*engine.Message{marker}, result...)
	}

	return result
}

// SnipByGroups is a grouping-aware variant of Snip.  It removes whole
// turn groups from the middle so that tool_use/tool_result pairs are
// never split.
func SnipByGroups(messages []*engine.Message, opts SnipOptions) []*engine.Message {
	if opts.RemoveThinking {
		messages = stripThinkingBlocks(messages)
	}

	keepFirst := opts.KeepFirstN
	if keepFirst <= 0 {
		keepFirst = 2
	}
	keepLast := opts.KeepLastN
	if keepLast <= 0 {
		keepLast = 6
	}

	if len(messages) <= keepFirst+keepLast {
		return messages
	}

	groups := GroupByTurns(messages)
	if len(groups) <= keepFirst+keepLast {
		return messages
	}

	// Determine group-level keep windows.
	headGroups := groups[:keepFirst]
	tailGroups := groups[len(groups)-keepLast:]

	// Reconstruct the message slice from kept groups.
	kept := make(map[int]bool)
	for _, g := range headGroups {
		kept[g.AssistantIdx] = true
		for _, ri := range g.ResultIdxs {
			kept[ri] = true
		}
	}
	for _, g := range tailGroups {
		kept[g.AssistantIdx] = true
		for _, ri := range g.ResultIdxs {
			kept[ri] = true
		}
	}

	out := make([]*engine.Message, 0, len(kept))
	for i, m := range messages {
		if kept[i] {
			out = append(out, m)
		}
	}

	if opts.MaxMessagesToKeep > 0 && len(out) > opts.MaxMessagesToKeep {
		out = out[len(out)-opts.MaxMessagesToKeep:]
	}

	return out
}
