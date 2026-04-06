package ccr

import (
	"sync"
	"time"
)

// ─── StreamBuffer ───────────────────────────────────────────────────────────
// Buffers text_delta events and merges them before upload to reduce API calls.
// Aligned with claude-code-main ccrClient.ts stream event buffering.

const (
	streamFlushInterval = 100 * time.Millisecond
	maxDeltaBufferLen   = 64 * 1024 // 64KB per message scope
)

// StreamBuffer accumulates text_delta events per message scope and flushes
// merged events at a configurable interval.
type StreamBuffer struct {
	mu       sync.Mutex
	scopes   map[string]*scopeBuffer // keyed by message scope ID
	onFlush  func(events []StreamEvent)
	interval time.Duration
	stopCh   chan struct{}
}

type scopeBuffer struct {
	text      string
	agentID   string
	firstTime int64
	lastTime  int64
}

// NewStreamBuffer creates a new StreamBuffer with the given flush callback.
func NewStreamBuffer(onFlush func(events []StreamEvent)) *StreamBuffer {
	return &StreamBuffer{
		scopes:   make(map[string]*scopeBuffer),
		onFlush:  onFlush,
		interval: streamFlushInterval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic flush loop.
func (sb *StreamBuffer) Start() {
	go func() {
		ticker := time.NewTicker(sb.interval)
		defer ticker.Stop()
		for {
			select {
			case <-sb.stopCh:
				sb.flush()
				return
			case <-ticker.C:
				sb.flush()
			}
		}
	}()
}

// AddTextDelta accumulates a text_delta event for the given message scope.
func (sb *StreamBuffer) AddTextDelta(scopeID, text, agentID string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	buf, ok := sb.scopes[scopeID]
	if !ok {
		buf = &scopeBuffer{
			agentID:   agentID,
			firstTime: time.Now().UnixMilli(),
		}
		sb.scopes[scopeID] = buf
	}

	// Truncate if buffer is too large
	if len(buf.text)+len(text) > maxDeltaBufferLen {
		return
	}

	buf.text += text
	buf.lastTime = time.Now().UnixMilli()
}

// AddEvent adds a non-text_delta event (passed through immediately via onFlush).
func (sb *StreamBuffer) AddEvent(evt StreamEvent) {
	if sb.onFlush != nil {
		sb.onFlush([]StreamEvent{evt})
	}
}

// Stop stops the flush loop and performs a final flush.
func (sb *StreamBuffer) Stop() {
	select {
	case <-sb.stopCh:
	default:
		close(sb.stopCh)
	}
}

func (sb *StreamBuffer) flush() {
	sb.mu.Lock()
	if len(sb.scopes) == 0 {
		sb.mu.Unlock()
		return
	}

	events := make([]StreamEvent, 0, len(sb.scopes))
	for scopeID, buf := range sb.scopes {
		if buf.text == "" {
			continue
		}
		events = append(events, StreamEvent{
			Type: "text_delta_merged",
			Payload: map[string]interface{}{
				"scope_id": scopeID,
				"text":     buf.text,
			},
			Timestamp: buf.lastTime,
			AgentID:   buf.agentID,
		})
	}
	// Clear all scopes after flush
	sb.scopes = make(map[string]*scopeBuffer)
	sb.mu.Unlock()

	if len(events) > 0 && sb.onFlush != nil {
		sb.onFlush(events)
	}
}
