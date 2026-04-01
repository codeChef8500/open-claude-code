package session

import (
	"sort"
	"strings"
	"time"
)

// SessionMetadata is already defined in types.go — this file adds index helpers.

// Search filters sessions by a keyword appearing in the title or work dir.
func (s *Storage) Search(keyword string) ([]*SessionMetadata, error) {
	all, err := s.ListSessions()
	if err != nil {
		return nil, err
	}
	kw := strings.ToLower(keyword)
	var results []*SessionMetadata
	for _, m := range all {
		if strings.Contains(strings.ToLower(m.Summary), kw) ||
			strings.Contains(strings.ToLower(m.WorkDir), kw) {
			results = append(results, m)
		}
	}
	return results, nil
}

// Recent returns up to n most recently updated sessions.
func (s *Storage) Recent(n int) ([]*SessionMetadata, error) {
	all, err := s.ListSessions()
	if err != nil {
		return nil, err
	}
	if n > 0 && len(all) > n {
		return all[:n], nil
	}
	return all, nil
}

// SortByCreated sorts a slice of metadata by CreatedAt descending (newest first).
func SortByCreated(metas []*SessionMetadata) {
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})
}

// Stats holds aggregate statistics across all sessions.
type Stats struct {
	TotalSessions int
	TotalTurns    int
	TotalTokens   int
	TotalCostUSD  float64
	OldestAt      time.Time
	NewestAt      time.Time
}

// CollectStats computes aggregate statistics from session metadata.
func CollectStats(metas []*SessionMetadata) Stats {
	if len(metas) == 0 {
		return Stats{}
	}
	s := Stats{
		TotalSessions: len(metas),
		OldestAt:      metas[0].CreatedAt,
		NewestAt:      metas[0].CreatedAt,
	}
	for _, m := range metas {
		s.TotalTurns += m.TurnCount
		s.TotalTokens += m.TotalTokens
		s.TotalCostUSD += m.CostUSD
		if m.CreatedAt.Before(s.OldestAt) {
			s.OldestAt = m.CreatedAt
		}
		if m.CreatedAt.After(s.NewestAt) {
			s.NewestAt = m.CreatedAt
		}
	}
	return s
}
