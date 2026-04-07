package session

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// ────────────────────────────────────────────────────────────────────────────
// Session restore — aligned with claude-code-main src/utils/sessionRestore.ts
// ────────────────────────────────────────────────────────────────────────────

// RestoreResult holds the result of restoring a session.
type RestoreResult struct {
	// Messages is the restored message history.
	Messages []*engine.Message
	// SummaryText is the compact summary (empty if no compaction happened).
	SummaryText string
	// Metadata is the restored session metadata.
	Metadata *SessionMetadata
	// WorkDir is the session's working directory.
	WorkDir string
	// Model is the model that was used in the session.
	Model string
	// TurnCount is the number of turns in the restored session.
	TurnCount int
	// TokensUsed is the total tokens used in the restored session.
	TokensUsed int
	// CostUSD is the total cost of the restored session.
	CostUSD float64
	// WasCompacted indicates whether the session had been compacted.
	WasCompacted bool
	// SessionAge is the time since the session was last updated.
	SessionAge time.Duration
	// OriginalMessageCount is the total transcript entries (before compaction trim).
	OriginalMessageCount int
}

// RestoreSession performs a full session restore, loading the transcript,
// metadata, and reconstructing the conversation state.
func (s *Storage) RestoreSession(sessionID string) (*RestoreResult, error) {
	// Load metadata
	meta, err := s.LoadMeta(sessionID)
	if err != nil {
		return nil, fmt.Errorf("restore: load metadata: %w", err)
	}
	if meta == nil {
		return nil, fmt.Errorf("restore: session %q not found", sessionID)
	}

	// Load transcript entries
	entries, err := s.ReadTranscript(sessionID)
	if err != nil {
		return nil, fmt.Errorf("restore: read transcript: %w", err)
	}

	result := &RestoreResult{
		Metadata:             meta,
		WorkDir:              meta.WorkDir,
		Model:                meta.Model,
		TurnCount:            meta.TurnCount,
		TokensUsed:           meta.TotalTokens,
		CostUSD:              meta.CostUSD,
		OriginalMessageCount: len(entries),
		SessionAge:           time.Since(meta.UpdatedAt),
	}

	// Find the last compact_summary boundary
	lastCompact := -1
	for i, e := range entries {
		if e.Type == EntryTypeCompactSummary {
			lastCompact = i
		}
	}

	start := 0
	if lastCompact >= 0 {
		result.WasCompacted = true
		start = lastCompact + 1
		if summary, ok := entries[lastCompact].Payload.(string); ok {
			result.SummaryText = summary
		}
	}

	// Reconstruct messages
	var messages []*engine.Message
	for _, e := range entries[start:] {
		if e.Type != EntryTypeMessage {
			continue
		}
		msg, err := payloadToMessage(e.Payload)
		if err != nil {
			slog.Debug("restore: skip unparseable message",
				slog.Any("err", err))
			continue
		}
		messages = append(messages, msg)
	}

	// If we have a summary but no messages after it, create synthetic messages
	if result.WasCompacted && len(messages) == 0 && result.SummaryText != "" {
		messages = []*engine.Message{
			{
				Role: engine.RoleUser,
				Content: []*engine.ContentBlock{{
					Type: engine.ContentTypeText,
					Text: "[Previous conversation summary]\n\n" + result.SummaryText,
				}},
			},
			{
				Role: engine.RoleAssistant,
				Content: []*engine.ContentBlock{{
					Type: engine.ContentTypeText,
					Text: "I have reviewed the conversation summary and am ready to continue.",
				}},
			},
		}
	}

	result.Messages = messages
	return result, nil
}

// ValidateRestore checks if a restored session is suitable for continuation.
// Returns a list of warnings (empty = good to go).
func ValidateRestore(result *RestoreResult) []string {
	var warnings []string

	if len(result.Messages) == 0 {
		warnings = append(warnings, "no messages to restore")
	}

	// Warn if session is very old
	if result.SessionAge > 7*24*time.Hour {
		warnings = append(warnings, fmt.Sprintf("session is %d days old, context may be stale",
			int(result.SessionAge.Hours()/24)))
	}

	// Warn if heavily compacted
	if result.WasCompacted && len(result.Messages) <= 2 {
		warnings = append(warnings, "session was heavily compacted, limited context available")
	}

	// Check for role alternation issues
	if len(result.Messages) >= 2 {
		for i := 1; i < len(result.Messages); i++ {
			if result.Messages[i].Role == result.Messages[i-1].Role {
				// Consecutive same-role messages (except assistant tool use pairs)
				if result.Messages[i].Role == engine.RoleUser {
					warnings = append(warnings, "consecutive user messages detected, may cause issues")
					break
				}
			}
		}
	}

	return warnings
}
