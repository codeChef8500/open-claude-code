package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ────────────────────────────────────────────────────────────────────────────
// Message factory functions (aligned with claude-code-main utils/messages.ts)
// ────────────────────────────────────────────────────────────────────────────

// CreateUserMessage builds a user message with proper UUID and timestamp.
func CreateUserMessage(text string, opts ...UserMessageOption) *Message {
	m := &Message{
		ID:        uuid.New().String(),
		UUID:      uuid.New().String(),
		Role:      RoleUser,
		Type:      MsgTypeUser,
		Content:   []*ContentBlock{{Type: ContentTypeText, Text: text}},
		Timestamp: time.Now(),
		Origin:    OriginUser,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// UserMessageOption is a functional option for CreateUserMessage.
type UserMessageOption func(*Message)

// WithMeta marks the message as meta (not shown to the user, e.g. system injected).
func WithMeta() UserMessageOption {
	return func(m *Message) { m.IsMeta = true }
}

// WithTranscriptOnly marks the message as visible only in transcript (not sent to API).
func WithTranscriptOnly() UserMessageOption {
	return func(m *Message) { m.IsVisibleInTranscriptOnly = true }
}

// WithToolUseResult attaches a tool-use-result reference to the user message.
func WithToolUseResult(toolUseID string) UserMessageOption {
	return func(m *Message) { m.ToolUseResult = toolUseID }
}

// WithSourceToolAssistant links the user message to the assistant message that
// triggered it (e.g. tool result messages).
func WithSourceToolAssistant(assistantUUID string) UserMessageOption {
	return func(m *Message) { m.SourceToolAssistantUUID = assistantUUID }
}

// WithImages adds base64-encoded images as content blocks.
func WithImages(images []string) UserMessageOption {
	return func(m *Message) {
		for _, img := range images {
			m.Content = append(m.Content, &ContentBlock{
				Type:      ContentTypeImage,
				MediaType: "image/png",
				Data:      img,
			})
		}
	}
}

// CreateAssistantMessage builds an assistant message from model response data.
func CreateAssistantMessage(content []*ContentBlock, usage *UsageStats, model string) *Message {
	m := &Message{
		ID:        uuid.New().String(),
		UUID:      uuid.New().String(),
		Role:      RoleAssistant,
		Type:      MsgTypeAssistant,
		Content:   content,
		Timestamp: time.Now(),
		Origin:    OriginAssistant,
		Model:     model,
	}
	if usage != nil {
		m.Usage = usage
		m.CostUSD = usage.CostUSD
	}
	return m
}

// CreateAssistantAPIErrorMessage builds a synthetic assistant message for API errors.
func CreateAssistantAPIErrorMessage(apiError, errorMsg string) *Message {
	return &Message{
		ID:        uuid.New().String(),
		UUID:      uuid.New().String(),
		Role:      RoleAssistant,
		Type:      MsgTypeSystemAPIError,
		Content:   []*ContentBlock{{Type: ContentTypeText, Text: errorMsg}},
		Timestamp: time.Now(),
		Origin:    OriginSystem,
		IsVirtual: true,
		APIError:  apiError,
		Error:     errorMsg,
		Level:     SystemLevelError,
	}
}

// CreateSystemMessage builds a system-level informational message.
func CreateSystemMessage(text string, level SystemMessageLevel) *Message {
	return &Message{
		ID:        uuid.New().String(),
		UUID:      uuid.New().String(),
		Role:      RoleSystem,
		Type:      MsgTypeSystemInformational,
		Content:   []*ContentBlock{{Type: ContentTypeText, Text: text}},
		Timestamp: time.Now(),
		Origin:    OriginSystem,
		Level:     level,
	}
}

// CreateProgressMessage builds a progress message for long-running operations.
func CreateProgressMessage(toolUseID, parentToolID, content, spinnerMode string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		UUID: uuid.New().String(),
		Role: RoleSystem,
		Type: MsgTypeProgress,
		ProgressData: &ProgressData{
			ToolUseID:    toolUseID,
			ParentToolID: parentToolID,
			Content:      content,
			SpinnerMode:  spinnerMode,
		},
		Timestamp: time.Now(),
		Origin:    OriginSystem,
	}
}

// CreateUserInterruptionMessage builds a synthetic user message indicating
// the user interrupted the assistant (e.g. Ctrl+C during tool use).
func CreateUserInterruptionMessage(toolUse bool) *Message {
	text := "The user has interrupted the response."
	if toolUse {
		text = "The user has interrupted the tool execution. Inform the user about what was done before the interruption."
	}
	return CreateUserMessage(text, WithMeta())
}

// CreateToolResultStopMessage builds a tool result block that instructs the
// model to stop execution (used when a stop hook fires).
func CreateToolResultStopMessage(toolUseID string) *ContentBlock {
	return &ContentBlock{
		Type:      ContentTypeToolResult,
		ToolUseID: toolUseID,
		Content: []*ContentBlock{{
			Type: ContentTypeText,
			Text: "The tool execution was stopped.",
		}},
		IsError: true,
	}
}

// CreateTombstoneMessage builds a tombstone that replaces a previous message in
// the UI/transcript (e.g. when a tool result is replaced by a summary).
func CreateTombstoneMessage(targetUUID, reason string) *Message {
	return &Message{
		ID:           uuid.New().String(),
		UUID:         uuid.New().String(),
		Role:         RoleSystem,
		Type:         MsgTypeTombstone,
		TombstoneFor: targetUUID,
		Timestamp:    time.Now(),
		Origin:       OriginSystem,
	}
}

// CreateCompactBoundaryMessage builds a message marking a compaction boundary.
func CreateCompactBoundaryMessage(tokensFreed, tokensRemaining int, summaryMsgID string) *Message {
	return &Message{
		ID:               uuid.New().String(),
		UUID:             uuid.New().String(),
		Role:             RoleSystem,
		Type:             MsgTypeCompactBoundary,
		IsCompactSummary: true,
		Timestamp:        time.Now(),
		Origin:           OriginCompact,
	}
}

// CreateAttachmentMessage builds an attachment message (memory, hook output, etc.).
func CreateAttachmentMessage(att *AttachmentData) *Message {
	return &Message{
		ID:         uuid.New().String(),
		UUID:       uuid.New().String(),
		Role:       RoleSystem,
		Type:       MsgTypeAttachment,
		Attachment: att,
		Timestamp:  time.Now(),
		Origin:     OriginSystem,
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Message query helpers
// ────────────────────────────────────────────────────────────────────────────

// IsSyntheticMessage returns true for messages that are NOT sent to the API
// (progress, attachment, tombstone, tool_use_summary, compact_boundary, etc.).
func IsSyntheticMessage(m *Message) bool {
	switch m.Type {
	case MsgTypeProgress, MsgTypeAttachment, MsgTypeTombstone,
		MsgTypeToolUseSummary, MsgTypeCompactBoundary, MsgTypeMicrocompactBoundary,
		MsgTypeSystemStopHookSummary, MsgTypeSystemTurnDuration,
		MsgTypeSystemMemorySaved:
		return true
	}
	return false
}

// IsNotEmptyMessage returns true if the message carries meaningful content
// (i.e. at least one non-empty text block, or a tool use/result block).
func IsNotEmptyMessage(m *Message) bool {
	if IsSyntheticMessage(m) {
		return true // synthetic messages are always "not empty" for UI
	}
	for _, b := range m.Content {
		switch b.Type {
		case ContentTypeText:
			if strings.TrimSpace(b.Text) != "" {
				return true
			}
		case ContentTypeToolUse, ContentTypeToolResult, ContentTypeImage,
			ContentTypeThinking, ContentTypeDocument:
			return true
		}
	}
	return false
}

// GetLastAssistantMessage returns the last assistant message in the slice,
// or nil if there is none.
func GetLastAssistantMessage(msgs []*Message) *Message {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleAssistant && msgs[i].Type == MsgTypeAssistant {
			return msgs[i]
		}
	}
	return nil
}

// HasToolCallsInLastAssistantTurn returns true if the last assistant turn
// contains at least one tool_use block.
func HasToolCallsInLastAssistantTurn(msgs []*Message) bool {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role == RoleAssistant {
			for _, b := range m.Content {
				if b.Type == ContentTypeToolUse {
					return true
				}
			}
			return false
		}
	}
	return false
}

// CountToolCalls returns the total number of tool_use blocks across all messages.
func CountToolCalls(msgs []*Message) int {
	count := 0
	for _, m := range msgs {
		for _, b := range m.Content {
			if b.Type == ContentTypeToolUse {
				count++
			}
		}
	}
	return count
}

// GetMessagesAfterCompactBoundary returns all messages after the last compact
// boundary message. If there is no compact boundary, returns all messages.
func GetMessagesAfterCompactBoundary(msgs []*Message) []*Message {
	lastBoundary := -1
	for i, m := range msgs {
		if m.Type == MsgTypeCompactBoundary {
			lastBoundary = i
		}
	}
	if lastBoundary < 0 {
		return msgs
	}
	return msgs[lastBoundary+1:]
}

// NormalizeMessagesForAPI strips synthetic messages, merges consecutive same-role
// turns, and validates tool-result pairing for API submission.
func NormalizeMessagesForAPI(msgs []*Message) []*Message {
	result := make([]*Message, 0, len(msgs))

	for _, m := range msgs {
		// Skip synthetic messages.
		if IsSyntheticMessage(m) {
			continue
		}
		// Skip transcript-only messages.
		if m.IsVisibleInTranscriptOnly {
			continue
		}
		result = append(result, m)
	}

	// Merge consecutive user messages into one (API requirement).
	merged := make([]*Message, 0, len(result))
	for _, m := range result {
		if len(merged) > 0 && merged[len(merged)-1].Role == m.Role && m.Role == RoleUser {
			last := merged[len(merged)-1]
			last.Content = append(last.Content, m.Content...)
		} else {
			merged = append(merged, m)
		}
	}

	return merged
}

// StripSignatureBlocks removes thinking signature blocks from content to
// reduce API token usage on re-sends.
func StripSignatureBlocks(content []*ContentBlock) []*ContentBlock {
	result := make([]*ContentBlock, 0, len(content))
	for _, b := range content {
		if b.Type == ContentTypeThinking && b.Signature != "" {
			// Keep thinking text but remove signature.
			stripped := *b
			stripped.Signature = ""
			result = append(result, &stripped)
		} else {
			result = append(result, b)
		}
	}
	return result
}

// ────────────────────────────────────────────────────────────────────────────
// Error detection helpers
// ────────────────────────────────────────────────────────────────────────────

// IsPromptTooLongError returns true if the error string indicates the prompt
// exceeds the model's context window.
func IsPromptTooLongError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	return strings.Contains(lower, "prompt is too long") ||
		strings.Contains(lower, "prompt_too_long") ||
		strings.Contains(lower, "context_length_exceeded") ||
		strings.Contains(lower, "maximum context length")
}

// IsMaxOutputTokensError returns true if the error/stop-reason indicates the
// response was truncated due to max_tokens.
func IsMaxOutputTokensError(stopReason string) bool {
	return stopReason == "max_tokens" || stopReason == "length"
}

// ────────────────────────────────────────────────────────────────────────────
// Denial / rejection message builders
// ────────────────────────────────────────────────────────────────────────────

const DenialWorkaroundGuidance = `If you believe this capability is essential to complete the user's request, STOP and explain to the user ` +
	`what you were trying to do and why you need this permission. Let the user decide how to proceed.`

// AutoRejectMessage builds the denial message shown when auto-mode blocks a tool.
func AutoRejectMessage(toolName string) string {
	return fmt.Sprintf("Permission to use %s has been denied. %s", toolName, DenialWorkaroundGuidance)
}

// DontAskRejectMessage builds the denial message for don't-ask mode.
func DontAskRejectMessage(toolName string) string {
	return fmt.Sprintf("Permission to use %s has been denied because the agent is running in don't ask mode. %s", toolName, DenialWorkaroundGuidance)
}

// ────────────────────────────────────────────────────────────────────────────
// UUID derivation (deterministic, for message normalization)
// ────────────────────────────────────────────────────────────────────────────

// DeriveUUID produces a stable UUID-shaped string from a parent UUID and an
// index. This is used when splitting a multi-block message into multiple
// single-block messages during normalization.
func DeriveUUID(parentUUID string, index int) string {
	hex := fmt.Sprintf("%012x", index)
	if len(parentUUID) >= 24 {
		return parentUUID[:24] + hex
	}
	return parentUUID + hex
}

// DeriveShortMessageID produces a short deterministic ID from a UUID, used for
// snip tool referencing.
func DeriveShortMessageID(u string) string {
	cleaned := strings.ReplaceAll(u, "-", "")
	if len(cleaned) < 10 {
		return cleaned
	}
	return cleaned[:10]
}
