package session

import "time"

// SessionMetadata describes a stored session without loading its full transcript.
type SessionMetadata struct {
	ID          string    `json:"id"`
	WorkDir     string    `json:"work_dir"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TurnCount   int       `json:"turn_count"`
	TotalTokens int       `json:"total_tokens"`
	CostUSD     float64   `json:"cost_usd"`
	Summary     string    `json:"summary,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Model       string    `json:"model,omitempty"`
	ForkOf      string    `json:"fork_of,omitempty"`
	ForkLabel   string    `json:"fork_label,omitempty"`
}

// TranscriptEntryType classifies each record in a JSONL transcript.
type TranscriptEntryType string

const (
	EntryTypeMessage        TranscriptEntryType = "message"
	EntryTypeToolUse        TranscriptEntryType = "tool_use"
	EntryTypeToolResult     TranscriptEntryType = "tool_result"
	EntryTypeCompactSummary TranscriptEntryType = "compact_summary"
	EntryTypeMetadata       TranscriptEntryType = "metadata"
)

// TranscriptEntry is one line in the JSONL session transcript.
type TranscriptEntry struct {
	Type      TranscriptEntryType `json:"type"`
	SessionID string              `json:"session_id"`
	Timestamp time.Time           `json:"timestamp"`
	Payload   interface{}         `json:"payload"`
}
