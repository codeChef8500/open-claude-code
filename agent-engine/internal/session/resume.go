package session

import (
	"encoding/json"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// Resume loads a session transcript and returns the messages suitable for
// re-injecting into a new engine conversation.  It skips compact_summary
// entries before the last compact boundary so the engine starts from a
// compacted context when available.
func (s *Storage) Resume(sessionID string) ([]*engine.Message, string, error) {
	entries, err := s.ReadTranscript(sessionID)
	if err != nil {
		return nil, "", err
	}

	// Find the last compact_summary boundary (if any).
	lastCompact := -1
	for i, e := range entries {
		if e.Type == EntryTypeCompactSummary {
			lastCompact = i
		}
	}

	// Build message slice from entries after (or from) the compact boundary.
	start := 0
	var summaryText string
	if lastCompact >= 0 {
		start = lastCompact + 1
		// The compact summary entry payload is a string summary.
		if s, ok := entries[lastCompact].Payload.(string); ok {
			summaryText = s
		}
	}

	var messages []*engine.Message
	for _, e := range entries[start:] {
		if e.Type != EntryTypeMessage {
			continue
		}
		msg, err := payloadToMessage(e.Payload)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	return messages, summaryText, nil
}

// payloadToMessage re-marshals an interface{} payload (decoded from JSON as
// map[string]interface{}) back into an *engine.Message.
func payloadToMessage(payload interface{}) (*engine.Message, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var msg engine.Message
	if err := json.Unmarshal(b, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
