package session

import (
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// SearchResult represents a match within a session transcript.
type SearchResult struct {
	SessionID string
	EntryIdx  int
	Role      string
	Snippet   string
	Score     float64
}

// SearchTranscript searches a session's transcript for entries matching query.
func (s *Storage) SearchTranscript(sessionID, query string) ([]SearchResult, error) {
	entries, err := s.ReadTranscript(sessionID)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []SearchResult

	for i, entry := range entries {
		if entry.Type != EntryTypeMessage {
			continue
		}
		msg, err := payloadToMessage(entry.Payload)
		if err != nil {
			continue
		}
		text := extractTextFromMessage(msg)
		lower := strings.ToLower(text)
		if !strings.Contains(lower, query) {
			continue
		}
		snippet := extractSnippet(text, query, 120)
		results = append(results, SearchResult{
			SessionID: sessionID,
			EntryIdx:  i,
			Role:      string(msg.Role),
			Snippet:   snippet,
			Score:     1.0,
		})
	}

	return results, nil
}

// SearchAllSessions searches across all sessions for matching entries.
func (s *Storage) SearchAllSessions(query string, maxResults int) ([]SearchResult, error) {
	metas, err := s.ListSessions()
	if err != nil {
		return nil, err
	}

	var all []SearchResult
	for _, meta := range metas {
		results, err := s.SearchTranscript(meta.ID, query)
		if err != nil {
			continue
		}
		all = append(all, results...)
		if maxResults > 0 && len(all) >= maxResults {
			all = all[:maxResults]
			break
		}
	}
	return all, nil
}

// SessionStats holds computed statistics about a session.
type SessionStats struct {
	TotalMessages    int
	UserMessages     int
	AssistantMessages int
	ToolCalls        int
	ToolErrors       int
	TotalTextChars   int
	CompactCount     int
}

// ComputeStats computes statistics for a session transcript.
func (s *Storage) ComputeStats(sessionID string) (*SessionStats, error) {
	entries, err := s.ReadTranscript(sessionID)
	if err != nil {
		return nil, err
	}

	stats := &SessionStats{}
	for _, entry := range entries {
		switch entry.Type {
		case EntryTypeMessage:
			stats.TotalMessages++
			msg, err := payloadToMessage(entry.Payload)
			if err != nil {
				continue
			}
			switch msg.Role {
			case engine.RoleUser:
				stats.UserMessages++
			case engine.RoleAssistant:
				stats.AssistantMessages++
				for _, b := range msg.Content {
					if b.Type == engine.ContentTypeToolUse {
						stats.ToolCalls++
					}
				}
			}
			stats.TotalTextChars += len(extractTextFromMessage(msg))
		case EntryTypeCompactSummary:
			stats.CompactCount++
		}
	}
	return stats, nil
}

func extractTextFromMessage(msg *engine.Message) string {
	var sb strings.Builder
	for _, b := range msg.Content {
		switch b.Type {
		case engine.ContentTypeText:
			sb.WriteString(b.Text)
		case engine.ContentTypeThinking:
			sb.WriteString(b.Thinking)
		}
	}
	return sb.String()
}

func extractSnippet(text, query string, maxLen int) string {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, strings.ToLower(query))
	if idx < 0 {
		if len(text) > maxLen {
			return text[:maxLen] + "..."
		}
		return text
	}

	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 80
	if end > len(text) {
		end = len(text)
	}

	snippet := text[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet = snippet + "..."
	}
	return snippet
}
