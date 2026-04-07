package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// LLM-based memory recall — aligned with claude-code-main
// src/memdir/findRelevantMemories.ts
// ────────────────────────────────────────────────────────────────────────────

// LLMCaller is a minimal interface for making LLM completion calls.
// Defined here to avoid importing the engine package (which would cause cycles).
type LLMCaller interface {
	// CompleteSimple makes a simple (non-streaming) completion call.
	// Returns the text content of the first response.
	CompleteSimple(ctx context.Context, systemPrompt, userMessage string) (string, error)
}

// FindRelevantMemories uses an LLM to select the most relevant memory files
// for a given user query from a list of memory headers.
// Returns the filenames of selected memories in order of relevance.
// Falls back to keyword-based scoring if the LLM call fails.
func FindRelevantMemories(
	ctx context.Context,
	caller LLMCaller,
	query string,
	headers []MemoryHeader,
	maxResults int,
) ([]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	if maxResults <= 0 {
		maxResults = 5
	}

	// Build the manifest for the LLM
	manifest := buildMemoryManifest(headers)

	systemPrompt := recallSystemPrompt
	userMessage := fmt.Sprintf("Query: %s\n\nAvailable memories:\n%s\n\nSelect up to %d relevant memories. Return JSON: {\"files\": [\"filename1.md\", ...]}",
		query, manifest, maxResults)

	// Try LLM-based recall
	if caller != nil {
		result, err := caller.CompleteSimple(ctx, systemPrompt, userMessage)
		if err != nil {
			slog.Warn("LLM recall failed, falling back to keyword scoring",
				slog.Any("err", err))
		} else {
			files := parseRecallResponse(result)
			if len(files) > 0 {
				// Validate filenames against known headers
				known := make(map[string]bool)
				for _, h := range headers {
					known[h.Filename] = true
				}
				var valid []string
				for _, f := range files {
					if known[f] && len(valid) < maxResults {
						valid = append(valid, f)
					}
				}
				if len(valid) > 0 {
					return valid, nil
				}
			}
		}
	}

	// Fallback: keyword-based scoring
	return keywordRecall(query, headers, maxResults), nil
}

// buildMemoryManifest formats headers into a manifest string for the LLM.
func buildMemoryManifest(headers []MemoryHeader) string {
	var sb strings.Builder
	for _, h := range headers {
		sb.WriteString(fmt.Sprintf("- `%s` [%s] — %s", h.Filename, string(h.Type), h.Description))
		if h.ModTimeMs > 0 {
			age := CalculateMemoryAge(timeFromMs(h.ModTimeMs))
			sb.WriteString(fmt.Sprintf(" (%s)", age))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// parseRecallResponse extracts filenames from the LLM JSON response.
func parseRecallResponse(raw string) []string {
	raw = strings.TrimSpace(raw)
	// Try to find JSON in the response
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return nil
	}
	jsonStr := raw[start : end+1]

	var resp struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil
	}
	return resp.Files
}

// keywordRecall performs simple keyword-based relevance scoring as a fallback.
func keywordRecall(query string, headers []MemoryHeader, maxResults int) []string {
	queryLower := strings.ToLower(query)
	words := strings.Fields(queryLower)

	type scored struct {
		filename string
		score    int
	}

	var results []scored
	for _, h := range headers {
		s := 0
		combined := strings.ToLower(h.Name + " " + h.Description + " " + h.Filename + " " + string(h.Type))
		for _, w := range words {
			if len(w) < 3 {
				continue
			}
			if strings.Contains(combined, w) {
				s++
			}
		}
		if s > 0 {
			results = append(results, scored{h.Filename, s})
		}
	}

	// Sort by score descending (simple insertion sort for small lists)
	for i := 1; i < len(results); i++ {
		j := i
		for j > 0 && results[j].score > results[j-1].score {
			results[j], results[j-1] = results[j-1], results[j]
			j--
		}
	}

	var out []string
	for i := 0; i < len(results) && i < maxResults; i++ {
		out = append(out, results[i].filename)
	}
	return out
}

func timeFromMs(ms int64) time.Time {
	return time.UnixMilli(ms)
}

const recallSystemPrompt = `You are a memory recall assistant. Your job is to select the most relevant memory files for a user's query from a list of available memories.

Rules:
- Select ONLY memories that are directly relevant to the query
- Prefer recent memories over old ones when relevance is similar
- Return a JSON object with a "files" array of filenames
- Do not include memories that are only tangentially related
- If no memories are relevant, return {"files": []}

Example response:
{"files": ["user_preferences.md", "project_architecture.md"]}`
