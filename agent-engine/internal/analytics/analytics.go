package analytics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventMetadata holds key-value pairs for an analytics event.
// Values must be primitive types (string, int, float64, bool) to avoid
// accidentally logging code or file contents.
type EventMetadata map[string]interface{}

// Sink is the interface that analytics backends must implement.
type Sink interface {
	LogEvent(eventName string, metadata EventMetadata)
	Flush() error
	Close() error
}

// queuedEvent is an event waiting for a sink to be attached.
type queuedEvent struct {
	Name     string
	Metadata EventMetadata
	Time     time.Time
}

var (
	mu         sync.Mutex
	activeSink Sink
	queue      []queuedEvent
	sessionID  string
	globalMeta EventMetadata
)

// SetSessionID sets a session ID that will be added to all events.
func SetSessionID(id string) {
	mu.Lock()
	sessionID = id
	mu.Unlock()
}

// SetGlobalMetadata sets metadata that will be merged into every event.
func SetGlobalMetadata(meta EventMetadata) {
	mu.Lock()
	globalMeta = meta
	mu.Unlock()
}

// AttachSink sets the active analytics sink and drains queued events.
// Idempotent: subsequent calls are no-ops.
func AttachSink(s Sink) {
	mu.Lock()
	defer mu.Unlock()

	if activeSink != nil {
		return
	}
	activeSink = s

	// Drain queue.
	for _, ev := range queue {
		activeSink.LogEvent(ev.Name, ev.Metadata)
	}
	queue = nil
}

// LogEvent logs an analytics event. If no sink is attached, the event
// is queued and will be drained when a sink is attached.
func LogEvent(eventName string, metadata EventMetadata) {
	mu.Lock()
	defer mu.Unlock()

	// Enrich with global metadata.
	enriched := enrichMetadata(metadata)

	if activeSink == nil {
		queue = append(queue, queuedEvent{
			Name:     eventName,
			Metadata: enriched,
			Time:     time.Now(),
		})
		return
	}
	activeSink.LogEvent(eventName, enriched)
}

// Flush flushes the active sink.
func Flush() error {
	mu.Lock()
	s := activeSink
	mu.Unlock()
	if s != nil {
		return s.Flush()
	}
	return nil
}

// Close flushes and closes the active sink.
func Close() error {
	mu.Lock()
	s := activeSink
	mu.Unlock()
	if s != nil {
		return s.Close()
	}
	return nil
}

func enrichMetadata(meta EventMetadata) EventMetadata {
	result := make(EventMetadata)
	// Global metadata first (can be overridden by event-specific).
	for k, v := range globalMeta {
		result[k] = v
	}
	for k, v := range meta {
		result[k] = v
	}
	if sessionID != "" {
		result["session_id"] = sessionID
	}
	result["timestamp"] = time.Now().UnixMilli()
	return result
}

// ── File Sink ─────────────────────────────────────────────────────────────

// FileSink writes events as JSONL to a file.
type FileSink struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileSink creates a FileSink that appends events to the given path.
func NewFileSink(path string) (*FileSink, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("analytics: mkdir %q: %w", dir, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("analytics: open %q: %w", path, err)
	}
	return &FileSink{file: f}, nil
}

func (s *FileSink) LogEvent(eventName string, metadata EventMetadata) {
	entry := make(map[string]interface{})
	entry["event"] = eventName
	for k, v := range metadata {
		entry[k] = v
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.file, "%s\n", data)
}

func (s *FileSink) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Sync()
}

func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Close()
}

// ── Nop Sink ──────────────────────────────────────────────────────────────

// NopSink discards all events (used when analytics is disabled).
type NopSink struct{}

func (NopSink) LogEvent(_ string, _ EventMetadata) {}
func (NopSink) Flush() error                       { return nil }
func (NopSink) Close() error                       { return nil }

// ── Multi Sink ────────────────────────────────────────────────────────────

// MultiSink fans out events to multiple sinks.
type MultiSink struct {
	sinks []Sink
}

// NewMultiSink creates a sink that dispatches to all provided sinks.
func NewMultiSink(sinks ...Sink) *MultiSink {
	return &MultiSink{sinks: sinks}
}

func (m *MultiSink) LogEvent(eventName string, metadata EventMetadata) {
	for _, s := range m.sinks {
		s.LogEvent(eventName, metadata)
	}
}

func (m *MultiSink) Flush() error {
	var firstErr error
	for _, s := range m.sinks {
		if err := s.Flush(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *MultiSink) Close() error {
	var firstErr error
	for _, s := range m.sinks {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// DefaultAnalyticsPath returns the default path for the analytics log.
func DefaultAnalyticsPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".claude", "analytics.jsonl")
}
