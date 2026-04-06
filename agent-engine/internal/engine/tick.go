package engine

import (
	"fmt"
	"time"
)

// TickTag is the XML tag name used for proactive tick messages.
// Aligned with claude-code-main KAIROS tick injection.
const TickTag = "tick"

// FormatTickMessage formats a proactive tick message as an XML element
// that the model recognizes as a heartbeat signal.
//
//	<tick count="5" timestamp="2024-03-15T10:00:00Z"/>
func FormatTickMessage(count int, ts time.Time) string {
	return fmt.Sprintf("<%s count=\"%d\" timestamp=\"%s\"/>",
		TickTag, count, ts.UTC().Format(time.RFC3339))
}

// ParseTickCount extracts the count attribute from a tick message.
// Returns -1 if the message is not a valid tick.
func ParseTickCount(msg string) int {
	var count int
	if _, err := fmt.Sscanf(msg, "<tick count=\"%d\"", &count); err != nil {
		return -1
	}
	return count
}
