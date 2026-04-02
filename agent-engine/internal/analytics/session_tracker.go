package analytics

import (
	"sync"
	"time"
)

// SessionTracker collects session-level metrics for analytics reporting.
type SessionTracker struct {
	mu sync.Mutex

	SessionID string
	StartTime time.Time
	Model     string
	WorkDir   string

	// Counters
	TotalTurns       int
	UserMessages     int
	AssistantMessages int
	ToolCalls        int
	ToolErrors       int
	CompactCount     int
	CommandCount     int

	// Token tracking
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCacheRead    int64
	TotalCacheWrite   int64

	// Cost
	TotalCostMicroUSD int64

	// Timing
	TotalAPILatencyMs int64
	APICallCount      int

	// Tool usage breakdown
	ToolUsage map[string]int
}

// NewSessionTracker creates a tracker for a new session.
func NewSessionTracker(sessionID, model, workDir string) *SessionTracker {
	return &SessionTracker{
		SessionID: sessionID,
		StartTime: time.Now(),
		Model:     model,
		WorkDir:   workDir,
		ToolUsage: make(map[string]int),
	}
}

// RecordTurn records a completed conversation turn.
func (t *SessionTracker) RecordTurn() {
	t.mu.Lock()
	t.TotalTurns++
	t.mu.Unlock()
}

// RecordUserMessage records a user message.
func (t *SessionTracker) RecordUserMessage() {
	t.mu.Lock()
	t.UserMessages++
	t.mu.Unlock()
}

// RecordAssistantMessage records an assistant message.
func (t *SessionTracker) RecordAssistantMessage() {
	t.mu.Lock()
	t.AssistantMessages++
	t.mu.Unlock()
}

// RecordToolCall records a tool invocation.
func (t *SessionTracker) RecordToolCall(toolName string, isError bool) {
	t.mu.Lock()
	t.ToolCalls++
	if isError {
		t.ToolErrors++
	}
	t.ToolUsage[toolName]++
	t.mu.Unlock()
}

// RecordCompact records a compaction event.
func (t *SessionTracker) RecordCompact() {
	t.mu.Lock()
	t.CompactCount++
	t.mu.Unlock()
}

// RecordCommand records a slash command execution.
func (t *SessionTracker) RecordCommand() {
	t.mu.Lock()
	t.CommandCount++
	t.mu.Unlock()
}

// RecordAPIUsage records token usage from an API call.
func (t *SessionTracker) RecordAPIUsage(input, output, cacheRead, cacheWrite int, costMicroUSD int64, latencyMs int64) {
	t.mu.Lock()
	t.TotalInputTokens += int64(input)
	t.TotalOutputTokens += int64(output)
	t.TotalCacheRead += int64(cacheRead)
	t.TotalCacheWrite += int64(cacheWrite)
	t.TotalCostMicroUSD += costMicroUSD
	t.TotalAPILatencyMs += latencyMs
	t.APICallCount++
	t.mu.Unlock()
}

// SessionSummary is a snapshot of session metrics.
type SessionSummary struct {
	SessionID         string         `json:"session_id"`
	Model             string         `json:"model"`
	DurationSeconds   float64        `json:"duration_seconds"`
	TotalTurns        int            `json:"total_turns"`
	UserMessages      int            `json:"user_messages"`
	AssistantMessages int            `json:"assistant_messages"`
	ToolCalls         int            `json:"tool_calls"`
	ToolErrors        int            `json:"tool_errors"`
	CompactCount      int            `json:"compact_count"`
	CommandCount      int            `json:"command_count"`
	InputTokens       int64          `json:"input_tokens"`
	OutputTokens      int64          `json:"output_tokens"`
	CacheReadTokens   int64          `json:"cache_read_tokens"`
	CacheWriteTokens  int64          `json:"cache_write_tokens"`
	TotalCostUSD      float64        `json:"total_cost_usd"`
	AvgAPILatencyMs   float64        `json:"avg_api_latency_ms"`
	APICallCount      int            `json:"api_call_count"`
	ToolUsage         map[string]int `json:"tool_usage"`
}

// Summary returns a snapshot of the current session metrics.
func (t *SessionTracker) Summary() SessionSummary {
	t.mu.Lock()
	defer t.mu.Unlock()

	duration := time.Since(t.StartTime).Seconds()
	avgLatency := 0.0
	if t.APICallCount > 0 {
		avgLatency = float64(t.TotalAPILatencyMs) / float64(t.APICallCount)
	}

	toolUsage := make(map[string]int, len(t.ToolUsage))
	for k, v := range t.ToolUsage {
		toolUsage[k] = v
	}

	return SessionSummary{
		SessionID:         t.SessionID,
		Model:             t.Model,
		DurationSeconds:   duration,
		TotalTurns:        t.TotalTurns,
		UserMessages:      t.UserMessages,
		AssistantMessages: t.AssistantMessages,
		ToolCalls:         t.ToolCalls,
		ToolErrors:        t.ToolErrors,
		CompactCount:      t.CompactCount,
		CommandCount:      t.CommandCount,
		InputTokens:       t.TotalInputTokens,
		OutputTokens:      t.TotalOutputTokens,
		CacheReadTokens:   t.TotalCacheRead,
		CacheWriteTokens:  t.TotalCacheWrite,
		TotalCostUSD:      float64(t.TotalCostMicroUSD) / 1_000_000,
		AvgAPILatencyMs:   avgLatency,
		APICallCount:      t.APICallCount,
		ToolUsage:         toolUsage,
	}
}

// EmitSessionEnd logs the session summary as an analytics event.
func (t *SessionTracker) EmitSessionEnd() {
	s := t.Summary()
	LogEvent("session_end", EventMetadata{
		"duration_seconds":   s.DurationSeconds,
		"total_turns":        s.TotalTurns,
		"user_messages":      s.UserMessages,
		"assistant_messages": s.AssistantMessages,
		"tool_calls":         s.ToolCalls,
		"tool_errors":        s.ToolErrors,
		"compact_count":      s.CompactCount,
		"command_count":      s.CommandCount,
		"input_tokens":       s.InputTokens,
		"output_tokens":      s.OutputTokens,
		"cache_read_tokens":  s.CacheReadTokens,
		"cache_write_tokens": s.CacheWriteTokens,
		"total_cost_usd":     s.TotalCostUSD,
		"avg_api_latency_ms": s.AvgAPILatencyMs,
		"api_call_count":     s.APICallCount,
		"model":              s.Model,
	})
}
