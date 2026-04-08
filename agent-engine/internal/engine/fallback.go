package engine

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// ────────────────────────────────────────────────────────────────────────────
// Fallback model switching & Tombstone mechanism.
// Aligned with claude-code-main query.ts inner try/catch for
// FallbackTriggeredError and the tombstone emit pattern.
// ────────────────────────────────────────────────────────────────────────────

// FallbackTriggeredError is raised during model streaming when a fallback
// model is configured and the primary model fails with a recoverable error
// (e.g. overloaded, rate-limited).
// Aligned with claude-code-main FallbackTriggeredError.
type FallbackTriggeredError struct {
	// OriginalModel is the model that failed.
	OriginalModel string
	// FallbackModel is the model to switch to.
	FallbackModel string
	// Reason is a human-readable description of why fallback was triggered.
	Reason string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *FallbackTriggeredError) Error() string {
	return fmt.Sprintf("fallback triggered: %s -> %s: %s", e.OriginalModel, e.FallbackModel, e.Reason)
}

// Unwrap returns the underlying error for errors.Is/As chain.
func (e *FallbackTriggeredError) Unwrap() error { return e.Err }

// IsFallbackTriggeredError reports whether err is (wraps) a FallbackTriggeredError.
func IsFallbackTriggeredError(err error) (*FallbackTriggeredError, bool) {
	if err == nil {
		return nil, false
	}
	// Unwrap chain.
	current := err
	for {
		if f, ok := current.(*FallbackTriggeredError); ok {
			return f, true
		}
		u, ok := current.(interface{ Unwrap() error })
		if !ok {
			return nil, false
		}
		current = u.Unwrap()
		if current == nil {
			return nil, false
		}
	}
}

// ── Tombstone emission ───────────────────────────────────────────────────

// EmitTombstones emits tombstone events for all messages that should be
// retracted from the UI (e.g. when a streaming fallback discards partial
// assistant output).
// Aligned with claude-code-main's tombstone yield pattern in query.ts.
func EmitTombstones(out chan<- *StreamEvent, messageUUIDs []string) {
	for _, msgUUID := range messageUUIDs {
		out <- &StreamEvent{
			Type: EventTombstone,
			Text: msgUUID,
		}
	}
}

// YieldMissingToolResultBlocks generates synthetic error tool_result messages
// for tool_use blocks that have no corresponding tool_result.
// This happens when streaming is aborted mid-turn (e.g. fallback or abort).
// Aligned with claude-code-main yieldMissingToolResultBlocks.
func YieldMissingToolResultBlocks(messages []*Message, out chan<- *StreamEvent) []*Message {
	// Collect all tool_use IDs from assistant messages.
	toolUseIDs := make(map[string]bool)
	for _, m := range messages {
		if m.Role != RoleAssistant {
			continue
		}
		for _, b := range m.Content {
			if b.Type == ContentTypeToolUse && b.ToolUseID != "" {
				toolUseIDs[b.ToolUseID] = true
			}
		}
	}

	// Collect all tool_result IDs from user messages.
	for _, m := range messages {
		if m.Role != RoleUser {
			continue
		}
		for _, b := range m.Content {
			if b.Type == ContentTypeToolResult && b.ToolUseID != "" {
				delete(toolUseIDs, b.ToolUseID)
			}
		}
	}

	// Generate synthetic error results for orphaned tool_use blocks.
	if len(toolUseIDs) == 0 {
		return messages
	}

	var blocks []*ContentBlock
	for toolUseID := range toolUseIDs {
		blocks = append(blocks, &ContentBlock{
			Type:      ContentTypeToolResult,
			ToolUseID: toolUseID,
			IsError:   true,
			Content: []*ContentBlock{{
				Type: ContentTypeText,
				Text: "Tool execution was interrupted by a model fallback or abort.",
			}},
		})
	}

	syntheticMsg := &Message{
		UUID:    uuid.New().String(),
		Role:    RoleUser,
		Type:    MsgTypeUser,
		IsMeta:  true,
		Content: blocks,
	}

	// Emit the synthetic message as a tool result event.
	for _, b := range syntheticMsg.Content {
		out <- &StreamEvent{
			Type:    EventToolResult,
			ToolID:  b.ToolUseID,
			Result:  "Tool execution was interrupted by a model fallback or abort.",
			IsError: true,
		}
	}

	return append(messages, syntheticMsg)
}

// ── Fallback recovery orchestration ──────────────────────────────────────

// FallbackRecoveryState tracks the state needed to recover from a fallback.
type FallbackRecoveryState struct {
	// OriginalModel is the model before fallback.
	OriginalModel string
	// FallbackModel is the model switched to.
	FallbackModel string
	// EmittedAssistantUUIDs are the UUIDs of assistant messages that need
	// tombstoning (they were from the failed original model attempt).
	EmittedAssistantUUIDs []string
	// EmittedToolUseIDs are tool_use IDs that were started but not completed.
	EmittedToolUseIDs []string
}

// HandleFallback processes a FallbackTriggeredError by:
//  1. Emitting tombstones for partial assistant messages.
//  2. Generating missing tool results for orphaned tool_use blocks.
//  3. Returning the cleaned-up messages and the new model to use.
func HandleFallback(
	fte *FallbackTriggeredError,
	messages []*Message,
	emittedAssistantUUIDs []string,
	out chan<- *StreamEvent,
) ([]*Message, string) {
	slog.Info("fallback: handling model switch",
		slog.String("from", fte.OriginalModel),
		slog.String("to", fte.FallbackModel),
		slog.String("reason", fte.Reason))

	// 1. Emit tombstones for partial assistant messages.
	EmitTombstones(out, emittedAssistantUUIDs)

	// 2. Remove the partial assistant messages from the message history.
	cleaned := removeMessagesByUUID(messages, emittedAssistantUUIDs)

	// 3. Generate missing tool results for any orphaned tool_use blocks.
	cleaned = YieldMissingToolResultBlocks(cleaned, out)

	// 4. Emit a system notification about the fallback.
	out <- &StreamEvent{
		Type: EventSystemMessage,
		Text: fmt.Sprintf("Switched from %s to %s: %s", fte.OriginalModel, fte.FallbackModel, fte.Reason),
	}

	return cleaned, fte.FallbackModel
}

// removeMessagesByUUID removes messages whose UUID is in the set.
func removeMessagesByUUID(messages []*Message, uuids []string) []*Message {
	if len(uuids) == 0 {
		return messages
	}
	set := make(map[string]bool, len(uuids))
	for _, u := range uuids {
		set[u] = true
	}
	result := make([]*Message, 0, len(messages))
	for _, m := range messages {
		if !set[m.UUID] {
			result = append(result, m)
		}
	}
	return result
}
