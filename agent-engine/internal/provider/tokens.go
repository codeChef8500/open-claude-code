package provider

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// EstimateTokens returns a rough token count estimate for a string.
// Uses a ~4 chars/token heuristic for English text, ~2 chars/token for CJK.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Count CJK characters for better estimation.
	cjkCount := 0
	totalChars := utf8.RuneCountInString(text)
	for _, r := range text {
		if isCJK(r) {
			cjkCount++
		}
	}
	nonCJK := totalChars - cjkCount
	// ~4 chars per token for latin, ~2 chars per token for CJK
	return (nonCJK+3)/4 + (cjkCount+1)/2
}

// EstimateMessageTokens estimates the total tokens across messages.
func EstimateMessageTokens(messages []*engine.Message) int {
	total := 0
	for _, m := range messages {
		// Role overhead: ~4 tokens
		total += 4
		for _, b := range m.Content {
			switch b.Type {
			case engine.ContentTypeText:
				total += EstimateTokens(b.Text)
			case engine.ContentTypeToolUse:
				total += EstimateTokens(b.ToolName) + 10
				if b.Input != nil {
					total += EstimateTokens(fmt.Sprintf("%v", b.Input))
				}
			case engine.ContentTypeToolResult:
				total += EstimateTokens(b.Text) + 10
			case engine.ContentTypeThinking:
				total += EstimateTokens(b.Text)
			case engine.ContentTypeImage:
				// Images are ~1600 tokens for a medium image
				total += 1600
			default:
				total += EstimateTokens(b.Text)
			}
		}
	}
	return total
}

// EstimateSystemPromptTokens estimates tokens for a system prompt string.
func EstimateSystemPromptTokens(prompt string) int {
	return EstimateTokens(prompt)
}

// TokenBudgetInfo provides a summary of token budget usage.
type TokenBudgetInfo struct {
	ContextWindowSize int
	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheWriteTokens  int
	UsedTokens        int
	RemainingTokens   int
	UsedFraction      float64
}

// ComputeTokenBudget calculates budget info from usage stats and model spec.
func ComputeTokenBudget(model string, usage *engine.UsageStats) TokenBudgetInfo {
	spec := ResolveModel(model)
	info := TokenBudgetInfo{
		ContextWindowSize: spec.ContextWindow,
	}
	if usage != nil {
		info.InputTokens = usage.InputTokens
		info.OutputTokens = usage.OutputTokens
		info.CacheReadTokens = usage.CacheReadInputTokens
		info.CacheWriteTokens = usage.CacheCreationInputTokens
		info.UsedTokens = usage.InputTokens + usage.OutputTokens
	}
	info.RemainingTokens = info.ContextWindowSize - info.UsedTokens
	if info.RemainingTokens < 0 {
		info.RemainingTokens = 0
	}
	if info.ContextWindowSize > 0 {
		info.UsedFraction = float64(info.UsedTokens) / float64(info.ContextWindowSize)
	}
	return info
}

// TruncateToTokenLimit truncates text to approximately maxTokens.
func TruncateToTokenLimit(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return text
	}
	estimated := EstimateTokens(text)
	if estimated <= maxTokens {
		return text
	}
	// Approximate char limit: maxTokens * 4
	charLimit := maxTokens * 4
	if charLimit >= len(text) {
		return text
	}
	// Find a clean break point (newline or space).
	truncated := text[:charLimit]
	if idx := strings.LastIndexByte(truncated, '\n'); idx > charLimit*3/4 {
		return truncated[:idx] + "\n[... truncated ...]"
	}
	return truncated + "\n[... truncated ...]"
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0x3000 && r <= 0x303F) || // CJK Symbols
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Hangul
}
