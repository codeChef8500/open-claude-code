package token

import (
	"strings"
	"unicode/utf8"
)

// EstimationMethod is the algorithm used to estimate tokens.
type EstimationMethod int

const (
	// EstimateCharsDiv4 approximates 1 token ≈ 4 characters (English text).
	EstimateCharsDiv4 EstimationMethod = iota
	// EstimateWordBased approximates 1 token ≈ 0.75 words.
	EstimateWordBased
)

// DefaultMethod is the estimation method used when none is specified.
const DefaultMethod = EstimateCharsDiv4

// Estimate returns an approximate token count for the given text.
// This is intentionally fast and approximate — use for heuristics only,
// not for billing or hard limits.
func Estimate(text string, method EstimationMethod) int {
	switch method {
	case EstimateWordBased:
		return estimateWordBased(text)
	default:
		return estimateCharsDiv4(text)
	}
}

// EstimateDefault estimates using EstimateCharsDiv4.
func EstimateDefault(text string) int {
	return estimateCharsDiv4(text)
}

// EstimateMessages estimates the total token count for a slice of (role, text)
// message pairs, adding per-message overhead (≈ 4 tokens/message for
// Claude's format).
func EstimateMessages(messages []MessagePair) int {
	const overheadPerMessage = 4
	total := 0
	for _, m := range messages {
		total += Estimate(m.Content, DefaultMethod) + overheadPerMessage
	}
	return total
}

// MessagePair is a (role, content) tuple for token estimation purposes.
type MessagePair struct {
	Role    string
	Content string
}

// EstimateRemainingBudget returns how many tokens are left in a context window
// given current usage.  Returns 0 if over budget.
func EstimateRemainingBudget(contextWindowSize, usedTokens int) int {
	remaining := contextWindowSize - usedTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// FitsInWindow reports whether text with the given existing usage would fit
// inside contextWindowSize.
func FitsInWindow(text string, contextWindowSize, usedTokens int) bool {
	needed := EstimateDefault(text)
	return usedTokens+needed <= contextWindowSize
}

// ── Internal helpers ───────────────────────────────────────────────────────

func estimateCharsDiv4(text string) int {
	chars := utf8.RuneCountInString(text)
	if chars == 0 {
		return 0
	}
	est := chars / 4
	if est == 0 {
		return 1
	}
	return est
}

func estimateWordBased(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	// 1 token ≈ 0.75 words, so tokens ≈ words / 0.75 = words * 4 / 3
	est := (words * 4) / 3
	if est == 0 {
		return 1
	}
	return est
}

// TruncateToTokenBudget truncates text so it fits within maxTokens.
// Returns the (possibly truncated) text and whether it was truncated.
func TruncateToTokenBudget(text string, maxTokens int) (string, bool) {
	estimated := EstimateDefault(text)
	if estimated <= maxTokens {
		return text, false
	}
	// Approximate char limit: maxTokens * 4
	charLimit := maxTokens * 4
	runes := []rune(text)
	if len(runes) <= charLimit {
		return text, false
	}
	return string(runes[:charLimit]) + "\n[... truncated ...]", true
}
