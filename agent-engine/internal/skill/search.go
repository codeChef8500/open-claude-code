package skill

import (
	"sort"
	"strings"
)

// SearchResult pairs a skill with its relevance score.
type SearchResult struct {
	Skill *Skill
	Score float64
}

// Search returns skills matching the query string, sorted by relevance (desc).
// It performs simple keyword matching against name, description, and tags.
func Search(skills []*Skill, query string) []SearchResult {
	if query == "" {
		results := make([]SearchResult, len(skills))
		for i, s := range skills {
			results[i] = SearchResult{Skill: s, Score: 1.0}
		}
		return results
	}

	tokens := strings.Fields(strings.ToLower(query))
	var results []SearchResult

	for _, s := range skills {
		score := scoreSkill(s, tokens)
		if score > 0 {
			results = append(results, SearchResult{Skill: s, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

// TopSkills returns at most n skills matching query.
func TopSkills(skills []*Skill, query string, n int) []*Skill {
	results := Search(skills, query)
	out := make([]*Skill, 0, n)
	for i, r := range results {
		if i >= n {
			break
		}
		out = append(out, r.Skill)
	}
	return out
}

func scoreSkill(s *Skill, tokens []string) float64 {
	nameLower := strings.ToLower(s.Meta.Name)
	descLower := strings.ToLower(s.Meta.Description)

	var score float64
	for _, t := range tokens {
		if strings.Contains(nameLower, t) {
			score += 2.0 // name match is weighted higher
		}
		if strings.Contains(descLower, t) {
			score += 1.0
		}
		for _, tag := range s.Meta.Tags {
			if strings.Contains(strings.ToLower(tag), t) {
				score += 1.5
			}
		}
	}
	return score
}
