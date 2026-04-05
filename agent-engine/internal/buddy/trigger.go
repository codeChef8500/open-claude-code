package buddy

import "strings"

// TextRange represents a start..end byte range in a string.
type TextRange struct {
	Start int
	End   int
}

// FindBuddyTriggerPositions returns the byte ranges of all "/buddy" occurrences
// in the given text, for input syntax highlighting.
// Matches claude-code-main useBuddyNotification.tsx findBuddyTriggerPositions().
func FindBuddyTriggerPositions(text string) []TextRange {
	const trigger = "/buddy"
	var ranges []TextRange
	offset := 0
	for {
		idx := strings.Index(text[offset:], trigger)
		if idx < 0 {
			break
		}
		start := offset + idx
		end := start + len(trigger)
		ranges = append(ranges, TextRange{Start: start, End: end})
		offset = end
	}
	return ranges
}
