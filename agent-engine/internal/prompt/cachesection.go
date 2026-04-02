package prompt

import "strings"

// CacheSectionMarker is the directive that marks the boundary after which
// content should NOT be included in the prompt cache (i.e. it's too dynamic).
const CacheSectionMarker = "<!-- cache-break -->"

// SplitCacheSections splits a prompt text at every CacheSectionMarker line,
// returning the sections in order.  The first section is the most cache-stable
// and each subsequent section is progressively more dynamic.
//
// If no marker is present the entire text is returned as a single section.
func SplitCacheSections(text string) []string {
	if !strings.Contains(text, CacheSectionMarker) {
		return []string{text}
	}
	lines := strings.Split(text, "\n")
	var sections []string
	var current []string
	for _, line := range lines {
		if strings.TrimSpace(line) == CacheSectionMarker {
			sections = append(sections, strings.Join(current, "\n"))
			current = nil
		} else {
			current = append(current, line)
		}
	}
	sections = append(sections, strings.Join(current, "\n"))
	return sections
}

// BuildPartsFromSections converts a slice of cache sections into PromptParts
// with ascending CacheOrder values, marking only the first section as cache-
// hinted (most stable).
func BuildPartsFromSections(sections []string) []PromptPart {
	var parts []PromptPart
	for i, s := range sections {
		if strings.TrimSpace(s) == "" {
			continue
		}
		parts = append(parts, PromptPart{
			Content:   s,
			Order:     CacheOrderBasePrompt + CacheOrder(i),
			CacheHint: i == 0,
		})
	}
	return parts
}

// AddCacheBreak appends a CacheSectionMarker to a prompt string.
func AddCacheBreak(prompt string) string {
	return strings.TrimRight(prompt, "\n") + "\n\n" + CacheSectionMarker + "\n"
}
