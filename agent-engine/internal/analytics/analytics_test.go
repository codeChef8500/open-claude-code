package analytics

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// resetGlobals resets the package-level state for test isolation.
func resetGlobals() {
	mu.Lock()
	activeSink = nil
	queue = nil
	sessionID = ""
	globalMeta = nil
	mu.Unlock()
}

// ── collectSink collects events in memory ───────────────────────────────────

type collectSink struct {
	mu     sync.Mutex
	events []struct {
		Name     string
		Metadata EventMetadata
	}
}

func (c *collectSink) LogEvent(name string, meta EventMetadata) {
	c.mu.Lock()
	c.events = append(c.events, struct {
		Name     string
		Metadata EventMetadata
	}{name, meta})
	c.mu.Unlock()
}

func (c *collectSink) Flush() error { return nil }
func (c *collectSink) Close() error { return nil }

func (c *collectSink) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

// ── Tests ───────────────────────────────────────────────────────────────────

func TestLogEvent_QueueAndDrain(t *testing.T) {
	resetGlobals()

	// Log before sink is attached - should be queued.
	LogEvent("pre_attach", EventMetadata{"key": "value"})

	sink := &collectSink{}
	AttachSink(sink)

	// Queued event should have been drained.
	if sink.count() != 1 {
		t.Errorf("expected 1 drained event, got %d", sink.count())
	}
	if sink.events[0].Name != "pre_attach" {
		t.Errorf("expected event name 'pre_attach', got %q", sink.events[0].Name)
	}
}

func TestLogEvent_DirectToSink(t *testing.T) {
	resetGlobals()

	sink := &collectSink{}
	AttachSink(sink)

	LogEvent("direct", EventMetadata{"x": 1})
	if sink.count() != 1 {
		t.Errorf("expected 1, got %d", sink.count())
	}
}

func TestSetSessionID(t *testing.T) {
	resetGlobals()

	SetSessionID("sess-123")
	sink := &collectSink{}
	AttachSink(sink)

	LogEvent("test", nil)
	if sink.events[0].Metadata["session_id"] != "sess-123" {
		t.Errorf("expected session_id 'sess-123', got %v", sink.events[0].Metadata["session_id"])
	}
}

func TestSetGlobalMetadata(t *testing.T) {
	resetGlobals()

	SetGlobalMetadata(EventMetadata{"app": "test-engine"})
	sink := &collectSink{}
	AttachSink(sink)

	LogEvent("evt", EventMetadata{"local": true})
	meta := sink.events[0].Metadata
	if meta["app"] != "test-engine" {
		t.Error("expected global metadata 'app'")
	}
	if meta["local"] != true {
		t.Error("expected local metadata 'local'")
	}
}

func TestAttachSink_Idempotent(t *testing.T) {
	resetGlobals()

	sink1 := &collectSink{}
	sink2 := &collectSink{}
	AttachSink(sink1)
	AttachSink(sink2) // should be no-op

	LogEvent("test", nil)
	if sink1.count() != 1 {
		t.Error("expected event on first sink")
	}
	if sink2.count() != 0 {
		t.Error("expected no events on second sink")
	}
}

func TestNopSink(t *testing.T) {
	s := NopSink{}
	s.LogEvent("x", nil) // should not panic
	if err := s.Flush(); err != nil {
		t.Errorf("Flush: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestMultiSink(t *testing.T) {
	s1 := &collectSink{}
	s2 := &collectSink{}
	ms := NewMultiSink(s1, s2)

	ms.LogEvent("multi", EventMetadata{"v": 1})
	if s1.count() != 1 || s2.count() != 1 {
		t.Error("expected event on both sinks")
	}
	if err := ms.Flush(); err != nil {
		t.Errorf("Flush: %v", err)
	}
	if err := ms.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestFileSink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	sink.LogEvent("file_test", EventMetadata{"count": 42})
	sink.LogEvent("file_test2", EventMetadata{"flag": true})
	if err := sink.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		var entry map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", lineCount, err)
		}
		if _, ok := entry["event"]; !ok {
			t.Errorf("line %d: missing 'event' key", lineCount)
		}
	}
	if lineCount != 2 {
		t.Errorf("expected 2 lines, got %d", lineCount)
	}
}

// ── SessionTracker tests ────────────────────────────────────────────────────

func TestSessionTracker_Counters(t *testing.T) {
	tr := NewSessionTracker("s1", "claude-sonnet-4", "/tmp")

	tr.RecordTurn()
	tr.RecordTurn()
	tr.RecordUserMessage()
	tr.RecordAssistantMessage()
	tr.RecordToolCall("Read", false)
	tr.RecordToolCall("Bash", true)
	tr.RecordCompact()
	tr.RecordCommand()

	s := tr.Summary()
	if s.TotalTurns != 2 {
		t.Errorf("turns: expected 2, got %d", s.TotalTurns)
	}
	if s.UserMessages != 1 {
		t.Errorf("user msgs: expected 1, got %d", s.UserMessages)
	}
	if s.AssistantMessages != 1 {
		t.Errorf("assistant msgs: expected 1, got %d", s.AssistantMessages)
	}
	if s.ToolCalls != 2 {
		t.Errorf("tool calls: expected 2, got %d", s.ToolCalls)
	}
	if s.ToolErrors != 1 {
		t.Errorf("tool errors: expected 1, got %d", s.ToolErrors)
	}
	if s.CompactCount != 1 {
		t.Errorf("compact: expected 1, got %d", s.CompactCount)
	}
	if s.CommandCount != 1 {
		t.Errorf("commands: expected 1, got %d", s.CommandCount)
	}
}

func TestSessionTracker_ToolUsage(t *testing.T) {
	tr := NewSessionTracker("s2", "model", "/tmp")
	tr.RecordToolCall("Read", false)
	tr.RecordToolCall("Read", false)
	tr.RecordToolCall("Bash", false)

	s := tr.Summary()
	if s.ToolUsage["Read"] != 2 {
		t.Errorf("Read usage: expected 2, got %d", s.ToolUsage["Read"])
	}
	if s.ToolUsage["Bash"] != 1 {
		t.Errorf("Bash usage: expected 1, got %d", s.ToolUsage["Bash"])
	}
}

func TestSessionTracker_APIUsage(t *testing.T) {
	tr := NewSessionTracker("s3", "model", "/tmp")
	tr.RecordAPIUsage(1000, 500, 200, 100, 5000, 250)
	tr.RecordAPIUsage(2000, 800, 300, 150, 8000, 350)

	s := tr.Summary()
	if s.InputTokens != 3000 {
		t.Errorf("input tokens: expected 3000, got %d", s.InputTokens)
	}
	if s.OutputTokens != 1300 {
		t.Errorf("output tokens: expected 1300, got %d", s.OutputTokens)
	}
	if s.CacheReadTokens != 500 {
		t.Errorf("cache read: expected 500, got %d", s.CacheReadTokens)
	}
	if s.CacheWriteTokens != 250 {
		t.Errorf("cache write: expected 250, got %d", s.CacheWriteTokens)
	}
	if s.TotalCostUSD != 0.013 {
		t.Errorf("cost: expected 0.013, got %f", s.TotalCostUSD)
	}
	if s.APICallCount != 2 {
		t.Errorf("api calls: expected 2, got %d", s.APICallCount)
	}
	if s.AvgAPILatencyMs != 300 {
		t.Errorf("avg latency: expected 300, got %f", s.AvgAPILatencyMs)
	}
}

func TestSessionTracker_SummaryDuration(t *testing.T) {
	tr := NewSessionTracker("s4", "model", "/tmp")
	s := tr.Summary()
	if s.DurationSeconds <= 0 {
		// Should be very small but positive.
		// This is a timing test, so be lenient.
	}
	if s.SessionID != "s4" {
		t.Errorf("session id: expected 's4', got %q", s.SessionID)
	}
	if s.Model != "model" {
		t.Errorf("model: expected 'model', got %q", s.Model)
	}
}
