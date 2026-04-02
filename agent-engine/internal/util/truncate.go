package util

import (
	"strings"
	"unicode/utf8"
)

// TruncateString truncates s to maxRunes runes, appending suffix if truncated.
func TruncateString(s, suffix string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	cutAt := maxRunes - utf8.RuneCountInString(suffix)
	if cutAt < 0 {
		cutAt = 0
	}
	return string(runes[:cutAt]) + suffix
}

// TruncateLines returns at most maxLines lines from s, appending a count line
// when truncated.
func TruncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	kept := lines[:maxLines]
	omitted := len(lines) - maxLines
	kept = append(kept, strings.Repeat(".", 3)+" ("+itoa(omitted)+" more lines)")
	return strings.Join(kept, "\n")
}

// TruncateMiddle keeps the first and last portions of s, replacing the middle
// with a placeholder when s exceeds maxRunes.
func TruncateMiddle(s string, maxRunes int, placeholder string) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	if placeholder == "" {
		placeholder = "\n... [truncated] ...\n"
	}
	runes := []rune(s)
	half := (maxRunes - utf8.RuneCountInString(placeholder)) / 2
	if half < 1 {
		half = 1
	}
	return string(runes[:half]) + placeholder + string(runes[len(runes)-half:])
}

// TruncateBytes truncates a byte slice to maxBytes, appending suffix bytes.
func TruncateBytes(b []byte, maxBytes int, suffix string) []byte {
	if len(b) <= maxBytes {
		return b
	}
	cutAt := maxBytes - len(suffix)
	if cutAt < 0 {
		cutAt = 0
	}
	out := make([]byte, cutAt+len(suffix))
	copy(out, b[:cutAt])
	copy(out[cutAt:], suffix)
	return out
}

// FitInBudget returns the longest prefix of s (in runes) such that the
// estimated token count (chars/4) fits within tokenBudget.
func FitInBudget(s string, tokenBudget int) string {
	charBudget := tokenBudget * 4
	runes := []rune(s)
	if len(runes) <= charBudget {
		return s
	}
	return string(runes[:charBudget])
}

// itoa converts an int to its decimal string representation.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n >= 10 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	pos--
	buf[pos] = byte('0' + n)
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
