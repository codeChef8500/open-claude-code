package engine

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ────────────────────────────────────────────────────────────────────────────
// SDK message normalization & Yield chain.
// Aligned with claude-code-main query.ts yield* helpers and
// normalizeAssistantMessage / normalizeToolResultMessage.
// ────────────────────────────────────────────────────────────────────────────

// NormalizeAssistantMessage normalizes a raw API assistant response into the
// canonical Message format.  It:
//   - Strips empty thinking blocks
//   - Assigns UUIDs to content blocks
//   - Sets timestamps
//   - Extracts stop_reason into StopReason
//
// Aligned with claude-code-main normalizeAssistantMessage.
func NormalizeAssistantMessage(raw *Message) *Message {
	if raw == nil {
		return nil
	}

	if raw.UUID == "" {
		raw.UUID = uuid.New().String()
	}
	if raw.Timestamp.IsZero() {
		raw.Timestamp = time.Now()
	}
	raw.Role = RoleAssistant
	raw.Type = MsgTypeAssistant

	// Strip empty thinking blocks.
	raw.Content = StripSignatureBlocks(raw.Content)

	// Assign UUIDs to content blocks.
	for _, b := range raw.Content {
		if b.Type == ContentTypeToolUse && b.ToolUseID == "" {
			b.ToolUseID = uuid.New().String()
		}
	}

	return raw
}

// NormalizeToolResultMessage normalizes a tool result message before it's
// appended to the conversation.  Ensures role, type, and UUID are set.
// Aligned with claude-code-main normalizeToolResultMessage.
func NormalizeToolResultMessage(msg *Message) *Message {
	if msg == nil {
		return nil
	}
	if msg.UUID == "" {
		msg.UUID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	msg.Role = RoleUser
	if msg.Type == "" {
		msg.Type = MsgTypeUser
	}
	return msg
}

// ── Yield helpers — emit events to the SDK stream ────────────────────────

// YieldAssistantMessage emits a normalized assistant message to the stream.
// Returns the message for further processing (e.g. tool execution).
func YieldAssistantMessage(msg *Message, out chan<- *StreamEvent) *Message {
	msg = NormalizeAssistantMessage(msg)
	if msg == nil {
		return nil
	}

	// Emit text deltas for each text content block.
	for _, b := range msg.Content {
		switch b.Type {
		case ContentTypeText:
			if b.Text != "" {
				out <- &StreamEvent{
					Type:        EventTextDelta,
					Text:        b.Text,
					MessageUUID: msg.UUID,
				}
			}
		case ContentTypeThinking:
			if b.Text != "" {
				out <- &StreamEvent{
					Type:        EventThinking,
					Thinking:    b.Text,
					MessageUUID: msg.UUID,
				}
			}
		case ContentTypeToolUse:
			out <- &StreamEvent{
				Type:     EventToolUse,
				ToolName: b.ToolName,
				ToolID:   b.ToolUseID,
			}
		}
	}

	return msg
}

// YieldToolResult emits a tool result to the stream.
func YieldToolResult(toolUseID, toolName string, blocks []*ContentBlock, isError bool, out chan<- *StreamEvent) {
	resultText := ""
	for _, b := range blocks {
		if b.Type == ContentTypeText && b.Text != "" {
			if resultText != "" {
				resultText += "\n"
			}
			resultText += b.Text
		}
	}

	out <- &StreamEvent{
		Type:     EventToolResult,
		ToolName: toolName,
		ToolID:   toolUseID,
		Result:   resultText,
		IsError:  isError,
	}
}

// YieldSystemMessage emits a system message to the stream.
func YieldSystemMessage(text string, level string, out chan<- *StreamEvent) {
	out <- &StreamEvent{
		Type:  EventSystemMessage,
		Text:  text,
		Level: level,
	}
}

// YieldUsage emits a usage statistics event.
func YieldUsage(stats *UsageStats, out chan<- *StreamEvent) {
	if stats == nil {
		return
	}
	out <- &StreamEvent{
		Type:  EventUsage,
		Usage: stats,
	}
}

// YieldDone emits the terminal done event.
func YieldDone(out chan<- *StreamEvent) {
	out <- &StreamEvent{
		Type: EventDone,
	}
}

// YieldCompactBoundary emits a compact boundary event.
func YieldCompactBoundary(info *CompactBoundaryData, out chan<- *StreamEvent) {
	if info == nil {
		return
	}
	out <- &StreamEvent{
		Type:        EventCompactBoundary,
		CompactInfo: info,
	}
}

// ── Message extraction helpers ───────────────────────────────────────────

// ExtractToolUseBlocks returns all tool_use content blocks from a message.
func ExtractToolUseBlocks(msg *Message) []*ContentBlock {
	if msg == nil {
		return nil
	}
	var blocks []*ContentBlock
	for _, b := range msg.Content {
		if b.Type == ContentTypeToolUse {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

// HasToolUse returns true if the message contains any tool_use blocks.
func HasToolUse(msg *Message) bool {
	return len(ExtractToolUseBlocks(msg)) > 0
}

// ExtractAssistantText extracts concatenated text from an assistant message.
func ExtractAssistantText(msg *Message) string {
	if msg == nil {
		return ""
	}
	result := ""
	for _, b := range msg.Content {
		if b.Type == ContentTypeText && b.Text != "" {
			if result != "" {
				result += "\n"
			}
			result += b.Text
		}
	}
	return result
}

// ── Cumulative usage tracking ────────────────────────────────────────────

// CumulativeUsage tracks total usage across all turns in a query.
type CumulativeUsage struct {
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCacheRead    int
	TotalCacheWrite   int
	TotalCostUSD      float64
	TurnCount         int
}

// Add accumulates a single turn's usage.
func (u *CumulativeUsage) Add(stats *UsageStats) {
	if stats == nil {
		return
	}
	u.TotalInputTokens += stats.InputTokens
	u.TotalOutputTokens += stats.OutputTokens
	u.TotalCacheRead += stats.CacheReadInputTokens
	u.TotalCacheWrite += stats.CacheCreationInputTokens
	u.TotalCostUSD += stats.CostUSD
	u.TurnCount++

	slog.Debug("cumulative_usage",
		slog.Int("turn", u.TurnCount),
		slog.Int("total_input", u.TotalInputTokens),
		slog.Int("total_output", u.TotalOutputTokens),
		slog.Float64("total_cost", u.TotalCostUSD))
}

// ToUsageStats converts cumulative usage to a summary stats event.
func (u *CumulativeUsage) ToUsageStats() *UsageStats {
	return &UsageStats{
		InputTokens:              u.TotalInputTokens,
		OutputTokens:             u.TotalOutputTokens,
		CacheReadInputTokens:     u.TotalCacheRead,
		CacheCreationInputTokens: u.TotalCacheWrite,
		CostUSD:                  u.TotalCostUSD,
	}
}
