package buddy

import (
	"strings"
	"unicode"
)

// TextRange represents a start..end byte range in a string.
type TextRange struct {
	Start int
	End   int
}

// FindBuddyTriggerPositions returns the byte ranges of all "/buddy" occurrences
// in the given text, for input syntax highlighting.
// Matches claude-code-main useBuddyNotification.tsx findBuddyTriggerPositions()
// which uses /\/buddy\b/g — we enforce a word boundary after the trigger.
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
		// Word boundary check: next char (if any) must not be alphanumeric/underscore
		if end < len(text) {
			next := rune(text[end])
			if unicode.IsLetter(next) || unicode.IsDigit(next) || next == '_' {
				offset = end
				continue
			}
		}
		ranges = append(ranges, TextRange{Start: start, End: end})
		offset = end
	}
	return ranges
}
