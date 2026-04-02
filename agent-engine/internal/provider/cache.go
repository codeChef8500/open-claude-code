package provider

import (
	"log/slog"
	"sync"
)

// CacheBreakReason describes why a prompt cache was invalidated.
type CacheBreakReason string

const (
	CacheBreakToolsChanged  CacheBreakReason = "tools_changed"
	CacheBreakSystemChanged CacheBreakReason = "system_changed"
	CacheBreakModelChanged  CacheBreakReason = "model_changed"
	CacheBreakExplicit      CacheBreakReason = "explicit"
)

// CacheStats tracks token usage across prompt cache interactions.
type CacheStats struct {
	mu sync.Mutex

	TotalInputTokens      int64
	TotalOutputTokens     int64
	CacheReadTokens       int64
	CacheWriteTokens      int64
	Requests              int64
	CacheHits             int64
	CacheMisses           int64
}

// Record accumulates a single API call's usage into the stats.
func (s *CacheStats) Record(inputTokens, outputTokens, cacheRead, cacheWrite int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Requests++
	s.TotalInputTokens += int64(inputTokens)
	s.TotalOutputTokens += int64(outputTokens)
	s.CacheReadTokens += int64(cacheRead)
	s.CacheWriteTokens += int64(cacheWrite)
	if cacheRead > 0 {
		s.CacheHits++
	} else {
		s.CacheMisses++
	}
}

// HitRate returns the fraction of requests that had at least one cache-read token.
func (s *CacheStats) HitRate() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Requests == 0 {
		return 0
	}
	return float64(s.CacheHits) / float64(s.Requests)
}

// Snapshot returns a copy of the current stats.
func (s *CacheStats) Snapshot() CacheStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return CacheStats{
		TotalInputTokens:  s.TotalInputTokens,
		TotalOutputTokens: s.TotalOutputTokens,
		CacheReadTokens:   s.CacheReadTokens,
		CacheWriteTokens:  s.CacheWriteTokens,
		Requests:          s.Requests,
		CacheHits:         s.CacheHits,
		CacheMisses:       s.CacheMisses,
	}
}

// PromptCacheBreakDetector detects when the prompt cache will be invalidated
// by comparing fingerprints of prompt segments across successive calls.
type PromptCacheBreakDetector struct {
	mu             sync.Mutex
	lastModel      string
	lastSystemHash uint64
	lastToolsHash  uint64
}

// Check compares the current call's hashes against the previous call.
// Returns the break reason if the cache is invalidated, or "" if still valid.
func (d *PromptCacheBreakDetector) Check(model string, systemHash, toolsHash uint64) CacheBreakReason {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.lastModel != model {
		d.update(model, systemHash, toolsHash)
		return CacheBreakModelChanged
	}
	if d.lastSystemHash != systemHash {
		d.update(model, systemHash, toolsHash)
		return CacheBreakSystemChanged
	}
	if d.lastToolsHash != toolsHash {
		d.update(model, systemHash, toolsHash)
		return CacheBreakToolsChanged
	}
	return ""
}

func (d *PromptCacheBreakDetector) update(model string, sysHash, toolsHash uint64) {
	d.lastModel = model
	d.lastSystemHash = sysHash
	d.lastToolsHash = toolsHash
}

// LogBreak logs a cache break event at debug level.
func LogBreak(reason CacheBreakReason) {
	if reason == "" {
		return
	}
	slog.Debug("prompt cache break", slog.String("reason", string(reason)))
}

// HashString is a fast non-cryptographic string hash (FNV-1a).
func HashString(s string) uint64 {
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)
	h := offset64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}
