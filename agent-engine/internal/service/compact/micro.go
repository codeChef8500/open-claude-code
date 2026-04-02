package compact

import (
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// compactableTools is the set of tool names whose results can be micro-compacted.
// Aligned with claude-code-main COMPACTABLE_TOOLS.
var compactableTools = map[string]bool{
	"Read":      true,
	"Bash":      true,
	"Grep":      true,
	"Glob":      true,
	"WebSearch": true,
	"WebFetch":  true,
	"Edit":      true,
	"Write":     true,
}

const (
	// TimeBasedMCClearedMessage is the replacement text for old cleared tool results.
	TimeBasedMCClearedMessage = "[Old tool result content cleared]"
	// imageMaxTokenSize is the estimated token size for image blocks.
	imageMaxTokenSize = 2000
)

// MicroCompactConfig holds options for micro-compaction.
type MicroCompactConfig struct {
	// MaxBlockChars is the maximum characters per text block (default 8000).
	MaxBlockChars int
	// KeepLastN preserves the last N messages from compaction (default 4).
	KeepLastN int
	// ClearOldToolResults replaces old compactable tool results with a stub.
	ClearOldToolResults bool
}

// MicroCompact performs local, heuristic-based message compaction without an
// LLM call.  It removes duplicate tool results, truncates very long text
// blocks, collapses consecutive whitespace, and optionally clears old
// compactable tool results.
func MicroCompact(messages []*engine.Message, maxBlockChars int) []*engine.Message {
	return MicroCompactWithConfig(messages, MicroCompactConfig{
		MaxBlockChars: maxBlockChars,
	})
}

// MicroCompactWithConfig performs micro-compaction with full configuration.
func MicroCompactWithConfig(messages []*engine.Message, cfg MicroCompactConfig) []*engine.Message {
	if cfg.MaxBlockChars <= 0 {
		cfg.MaxBlockChars = 8000
	}
	if cfg.KeepLastN <= 0 {
		cfg.KeepLastN = 4
	}

	n := len(messages)
	protectedStart := n - cfg.KeepLastN
	if protectedStart < 0 {
		protectedStart = 0
	}

	// Track seen tool results for duplicate detection.
	seenToolResults := make(map[string]int) // toolName+hash -> index

	out := make([]*engine.Message, 0, n)
	for i, m := range messages {
		isProtected := i >= protectedStart
		compacted := compactMessageFull(m, cfg.MaxBlockChars, isProtected, cfg.ClearOldToolResults, seenToolResults, i)
		out = append(out, compacted)
	}
	return out
}

func compactMessageFull(
	m *engine.Message,
	maxBlockChars int,
	isProtected bool,
	clearOld bool,
	seenToolResults map[string]int,
	idx int,
) *engine.Message {
	newBlocks := make([]*engine.ContentBlock, 0, len(m.Content))
	for _, b := range m.Content {
		nb := compactBlockFull(b, maxBlockChars, isProtected, clearOld, seenToolResults, idx)
		newBlocks = append(newBlocks, nb)
	}
	return &engine.Message{
		ID:        m.ID,
		Role:      m.Role,
		Content:   newBlocks,
		Timestamp: m.Timestamp,
		SessionID: m.SessionID,
	}
}

func compactBlockFull(
	b *engine.ContentBlock,
	maxChars int,
	isProtected bool,
	clearOld bool,
	seenToolResults map[string]int,
	idx int,
) *engine.ContentBlock {
	switch b.Type {
	case engine.ContentTypeToolResult:
		return compactToolResult(b, maxChars, isProtected, clearOld, seenToolResults, idx)
	case engine.ContentTypeText:
		return compactTextBlock(b, maxChars)
	default:
		return b
	}
}

// compactToolResult handles micro-compaction of tool result blocks:
// - Clears old compactable tool results if configured
// - Detects and deduplicates repeated tool results
// - Truncates oversized nested text blocks
func compactToolResult(
	b *engine.ContentBlock,
	maxChars int,
	isProtected bool,
	clearOld bool,
	seenToolResults map[string]int,
	idx int,
) *engine.ContentBlock {
	toolName := b.ToolName

	// Clear old compactable tool results (not protected ones).
	if clearOld && !isProtected && compactableTools[toolName] {
		return &engine.ContentBlock{
			Type:      b.Type,
			ToolUseID: b.ToolUseID,
			ToolName:  b.ToolName,
			Content: []*engine.ContentBlock{{
				Type: engine.ContentTypeText,
				Text: TimeBasedMCClearedMessage,
			}},
			IsError: b.IsError,
		}
	}

	// Duplicate detection: if we've seen an identical tool result for the same
	// tool name, replace the older one with a stub.
	resultText := toolResultText(b)
	if resultText != "" && compactableTools[toolName] {
		key := toolName + ":" + hashStr(resultText)
		if _, exists := seenToolResults[key]; exists {
			// This is a duplicate — replace with stub.
			return &engine.ContentBlock{
				Type:      b.Type,
				ToolUseID: b.ToolUseID,
				ToolName:  b.ToolName,
				Content: []*engine.ContentBlock{{
					Type: engine.ContentTypeText,
					Text: fmt.Sprintf("[Duplicate %s result — see earlier occurrence]", toolName),
				}},
				IsError: b.IsError,
			}
		}
		seenToolResults[key] = idx
	}

	// Truncate oversized nested text blocks within tool results.
	newContent := make([]*engine.ContentBlock, 0, len(b.Content))
	for _, c := range b.Content {
		if c.Type == engine.ContentTypeText && len(c.Text) > maxChars {
			truncated := c.Text[:maxChars] + "\n... [truncated]"
			newContent = append(newContent, &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: truncated,
			})
		} else {
			newContent = append(newContent, c)
		}
	}

	return &engine.ContentBlock{
		Type:      b.Type,
		ToolUseID: b.ToolUseID,
		ToolName:  b.ToolName,
		Content:   newContent,
		IsError:   b.IsError,
	}
}

// compactTextBlock collapses whitespace and truncates oversized text blocks.
func compactTextBlock(b *engine.ContentBlock, maxChars int) *engine.ContentBlock {
	text := strings.Join(strings.Fields(b.Text), " ") // collapse whitespace
	if len(text) > maxChars {
		text = text[:maxChars] + "\n... [truncated]"
	}
	return &engine.ContentBlock{
		Type:    b.Type,
		Text:    text,
		IsError: b.IsError,
	}
}

// toolResultText extracts the concatenated text from a tool result block.
func toolResultText(b *engine.ContentBlock) string {
	var sb strings.Builder
	for _, c := range b.Content {
		if c.Type == engine.ContentTypeText {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

// hashStr returns a simple hash string for deduplication.
func hashStr(s string) string {
	// Use a simple FNV-like hash for speed.
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}

// EstimateToolResultTokens estimates the token count for a tool result block.
func EstimateToolResultTokens(b *engine.ContentBlock) int {
	if b.Type != engine.ContentTypeToolResult {
		return 0
	}
	total := 0
	for _, c := range b.Content {
		switch c.Type {
		case engine.ContentTypeText:
			total += len(c.Text) / 4
		case engine.ContentTypeImage:
			total += imageMaxTokenSize
		}
	}
	return total
}
