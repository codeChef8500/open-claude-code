package prompt

// CacheOrder defines the preferred ordering of system prompt parts to
// maximise Anthropic prompt cache hits. Stable parts come first.
type CacheOrder int

const (
	CacheOrderBasePrompt    CacheOrder = 0
	CacheOrderToolDescs     CacheOrder = 1
	CacheOrderMemories      CacheOrder = 2
	CacheOrderEnvironment   CacheOrder = 3
	CacheOrderCustomPrompt  CacheOrder = 4
	CacheOrderAppendPrompt  CacheOrder = 5
)

// PromptPart represents one segment of the composed system prompt.
type PromptPart struct {
	Content    string
	Order      CacheOrder
	CacheHint  bool // If true, inject cache_control=ephemeral on this block
}

// CacheStats tracks cache token usage returned by the Anthropic API.
type CacheStats struct {
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	// Estimated savings: cache read costs ~10% of normal input
	EstimatedSavingsUSD float64
}

// UpdateCacheStats accumulates cache token counts.
func UpdateCacheStats(s *CacheStats, creation, read int, inputCPMUSD float64) {
	s.CacheCreationInputTokens += creation
	s.CacheReadInputTokens += read
	// Normal cost for read tokens minus cache-read cost (10%)
	s.EstimatedSavingsUSD += float64(read) * inputCPMUSD / 1_000_000 * 0.90
}

// SortParts returns parts in cache-friendly order (stable → dynamic).
func SortParts(parts []PromptPart) []PromptPart {
	// Insertion sort is stable and adequate for the small slice (≤6 elements).
	for i := 1; i < len(parts); i++ {
		for j := i; j > 0 && parts[j].Order < parts[j-1].Order; j-- {
			parts[j], parts[j-1] = parts[j-1], parts[j]
		}
	}
	return parts
}
