package engine

import (
	"sync"
)

// ContentReplacementState tracks tool result budget and content replacement
// records for a conversation thread. When a tool result exceeds the budget,
// the full content is persisted to disk and replaced with a reference.
//
// Aligned with claude-code-main's contentReplacementState in query.ts.
type ContentReplacementState struct {
	mu sync.Mutex

	// Records maps tool-use-ID → replacement record.
	Records map[string]*ContentReplacementRecord

	// BudgetTokens is the total token budget for tool results in a turn.
	BudgetTokens int

	// UsedTokens tracks how many tokens have been consumed by tool results.
	UsedTokens int
}

// ContentReplacementRecord tracks a single tool result that was replaced.
type ContentReplacementRecord struct {
	ToolUseID    string `json:"tool_use_id"`
	ToolName     string `json:"tool_name"`
	OriginalSize int    `json:"original_size"`    // characters
	ReplacedSize int    `json:"replaced_size"`    // characters after replacement
	StoragePath  string `json:"storage_path"`     // path to persisted full content
	Replaced     bool   `json:"replaced"`
}

// NewContentReplacementState creates a state with the given token budget.
func NewContentReplacementState(budgetTokens int) *ContentReplacementState {
	if budgetTokens <= 0 {
		budgetTokens = 80000 // default ~80K tokens for tool results
	}
	return &ContentReplacementState{
		Records:      make(map[string]*ContentReplacementRecord),
		BudgetTokens: budgetTokens,
	}
}

// ShouldReplace reports whether a tool result of the given size (in estimated
// tokens) should be replaced to stay within budget.
func (s *ContentReplacementState) ShouldReplace(estimatedTokens int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.UsedTokens+estimatedTokens > s.BudgetTokens
}

// RecordUsage accounts for token usage from a tool result.
func (s *ContentReplacementState) RecordUsage(toolUseID string, estimatedTokens int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UsedTokens += estimatedTokens
	if rec, ok := s.Records[toolUseID]; ok {
		rec.Replaced = false
	}
}

// RecordReplacement records that a tool result was replaced.
func (s *ContentReplacementState) RecordReplacement(rec *ContentReplacementRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec.Replaced = true
	s.Records[rec.ToolUseID] = rec
}

// GetRecord returns the replacement record for a tool use, if any.
func (s *ContentReplacementState) GetRecord(toolUseID string) (*ContentReplacementRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.Records[toolUseID]
	return rec, ok
}

// RemainingBudget returns the remaining token budget.
func (s *ContentReplacementState) RemainingBudget() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	rem := s.BudgetTokens - s.UsedTokens
	if rem < 0 {
		return 0
	}
	return rem
}

// Clone creates a shallow copy of the state (for subagent forking).
func (s *ContentReplacementState) Clone() *ContentReplacementState {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make(map[string]*ContentReplacementRecord, len(s.Records))
	for k, v := range s.Records {
		clone := *v
		records[k] = &clone
	}
	return &ContentReplacementState{
		Records:      records,
		BudgetTokens: s.BudgetTokens,
		UsedTokens:   s.UsedTokens,
	}
}
