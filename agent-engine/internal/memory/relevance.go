package memory

import (
	"sort"
	"strings"
	"time"
)

// ScoredMemory pairs a memory item with a relevance score.
type ScoredMemory struct {
	Memory *ExtractedMemory
	Score  float64
}

// RankByRelevance ranks extracted memories by relevance to a query string,
// with a time-decay bonus for recently extracted memories.
// Returns memories sorted by descending score.
func RankByRelevance(memories []*ExtractedMemory, query string) []ScoredMemory {
	qTokens := tokenise(query)
	now := time.Now()

	scored := make([]ScoredMemory, 0, len(memories))
	for _, m := range memories {
		score := keywordScore(m.Content, qTokens)
		score += timeDecay(m.ExtractedAt, now)
		scored = append(scored, ScoredMemory{Memory: m, Score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	return scored
}

// TopN returns at most n memories ranked by relevance.
func TopN(memories []*ExtractedMemory, query string, n int) []*ExtractedMemory {
	ranked := RankByRelevance(memories, query)
	result := make([]*ExtractedMemory, 0, n)
	for i, sm := range ranked {
		if i >= n {
			break
		}
		result = append(result, sm.Memory)
	}
	return result
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func tokenise(s string) []string {
	s = strings.ToLower(s)
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	// De-duplicate
	seen := make(map[string]bool, len(words))
	var unique []string
	for _, w := range words {
		if len(w) > 2 && !seen[w] {
			seen[w] = true
			unique = append(unique, w)
		}
	}
	return unique
}

func keywordScore(content string, tokens []string) float64 {
	lower := strings.ToLower(content)
	var score float64
	for _, t := range tokens {
		if strings.Contains(lower, t) {
			score += 1.0
		}
	}
	return score
}

// timeDecay returns a bonus in [0, 0.5] for recently extracted memories.
// Memories extracted within the last hour get 0.5; within a day get 0.2; older get 0.
func timeDecay(extractedAt, now time.Time) float64 {
	age := now.Sub(extractedAt)
	switch {
	case age < time.Hour:
		return 0.5
	case age < 24*time.Hour:
		return 0.2
	case age < 7*24*time.Hour:
		return 0.1
	default:
		return 0
	}
}
