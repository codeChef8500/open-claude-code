package ccr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// ─── Constants ──────────────────────────────────────────────────────────────

const (
	DefaultHeartbeatInterval  = 20 * time.Second
	StreamEventFlushInterval  = 100 * time.Millisecond
	MaxRetries                = 3
	RetryBaseDelay            = 1 * time.Second
	MaxStreamEventBufferSize  = 1000
)

// ─── Types ──────────────────────────────────────────────────────────────────

// ClientConfig configures a CCR client instance.
type ClientConfig struct {
	// BaseURL is the CCR API endpoint (e.g. https://api.claude.ai/ccr/v1).
	BaseURL string
	// WorkerID uniquely identifies this worker.
	WorkerID string
	// SessionID is the current session identifier.
	SessionID string
	// AuthToken for API authentication.
	AuthToken string
	// HeartbeatInterval overrides the default heartbeat interval.
	HeartbeatInterval time.Duration
	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client
}

// WorkerRegistration is the response from worker registration.
type WorkerRegistration struct {
	WorkerID  string `json:"worker_id"`
	Epoch     int    `json:"epoch"`
	SessionID string `json:"session_id"`
}

// StreamEvent represents a buffered event for upload.
type StreamEvent struct {
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp int64                  `json:"timestamp"`
	AgentID   string                 `json:"agent_id,omitempty"`
}

// InternalEvent is a system-level event (compaction, state change, etc).
type InternalEvent struct {
	EventType    string                 `json:"event_type"`
	Payload      map[string]interface{} `json:"payload"`
	IsCompaction bool                   `json:"is_compaction,omitempty"`
	AgentID      string                 `json:"agent_id,omitempty"`
}

// ─── Client ─────────────────────────────────────────────────────────────────

// Client manages cloud worker lifecycle: registration, heartbeat,
// event upload, stream event buffering, and epoch management.
// Ported from claude-code-main cli/transports/ccrClient.ts.
type Client struct {
	cfg   ClientConfig
	http  *http.Client
	epoch int

	mu            sync.Mutex
	eventBuf      []StreamEvent
	flushTicker   *time.Ticker
	heartbeatStop context.CancelFunc
	registered    bool
}

// NewClient creates a new CCR client.
func NewClient(cfg ClientConfig) *Client {
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = DefaultHeartbeatInterval
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		cfg:      cfg,
		http:     httpClient,
		eventBuf: make([]StreamEvent, 0, 64),
	}
}

// Initialize registers this worker with the CCR backend and starts
// the heartbeat loop. Returns the initial configuration or nil.
func (c *Client) Initialize(ctx context.Context, epoch int) (map[string]interface{}, error) {
	c.epoch = epoch

	body := map[string]interface{}{
		"worker_id":  c.cfg.WorkerID,
		"session_id": c.cfg.SessionID,
		"epoch":      epoch,
	}

	resp, err := c.postJSON(ctx, "/workers/register", body)
	if err != nil {
		return nil, fmt.Errorf("register worker: %w", err)
	}

	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()

	// Start heartbeat loop
	hbCtx, cancel := context.WithCancel(ctx)
	c.heartbeatStop = cancel
	go c.heartbeatLoop(hbCtx)

	// Start stream event flush loop
	c.flushTicker = time.NewTicker(StreamEventFlushInterval)
	go c.flushLoop(hbCtx)

	slog.Info("ccr: worker registered",
		slog.String("worker_id", c.cfg.WorkerID),
		slog.Int("epoch", epoch))

	return resp, nil
}

// WriteEvent buffers a stream event for batched upload.
func (c *Client) WriteEvent(evType string, payload map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.eventBuf) >= MaxStreamEventBufferSize {
		// Drop oldest events when buffer is full
		c.eventBuf = c.eventBuf[1:]
	}

	c.eventBuf = append(c.eventBuf, StreamEvent{
		Type:      evType,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	})
}

// WriteInternalEvent sends a system-level event immediately.
func (c *Client) WriteInternalEvent(ctx context.Context, evt InternalEvent) error {
	body := map[string]interface{}{
		"worker_id":  c.cfg.WorkerID,
		"session_id": c.cfg.SessionID,
		"epoch":      c.epoch,
		"event_type": evt.EventType,
		"payload":    evt.Payload,
	}
	if evt.IsCompaction {
		body["is_compaction"] = true
	}
	if evt.AgentID != "" {
		body["agent_id"] = evt.AgentID
	}

	_, err := c.postJSON(ctx, "/workers/events/internal", body)
	return err
}

// Epoch returns the current epoch.
func (c *Client) Epoch() int {
	return c.epoch
}

// SetEpoch updates the current epoch (e.g. after a restart).
func (c *Client) SetEpoch(epoch int) {
	c.epoch = epoch
}

// Close stops the heartbeat and flushes remaining events.
func (c *Client) Close(ctx context.Context) error {
	if c.heartbeatStop != nil {
		c.heartbeatStop()
	}
	if c.flushTicker != nil {
		c.flushTicker.Stop()
	}
	// Final flush
	return c.flushEvents(ctx)
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.sendHeartbeat(ctx); err != nil {
				slog.Warn("ccr: heartbeat failed", slog.Any("err", err))
			}
		}
	}
}

func (c *Client) sendHeartbeat(ctx context.Context) error {
	body := map[string]interface{}{
		"worker_id":  c.cfg.WorkerID,
		"session_id": c.cfg.SessionID,
		"epoch":      c.epoch,
		"timestamp":  time.Now().UnixMilli(),
	}
	_, err := c.postJSON(ctx, "/workers/heartbeat", body)
	return err
}

func (c *Client) flushLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.flushTicker.C:
			if err := c.flushEvents(ctx); err != nil {
				slog.Debug("ccr: flush failed", slog.Any("err", err))
			}
		}
	}
}

func (c *Client) flushEvents(ctx context.Context) error {
	c.mu.Lock()
	if len(c.eventBuf) == 0 {
		c.mu.Unlock()
		return nil
	}
	events := make([]StreamEvent, len(c.eventBuf))
	copy(events, c.eventBuf)
	c.eventBuf = c.eventBuf[:0]
	c.mu.Unlock()

	body := map[string]interface{}{
		"worker_id":  c.cfg.WorkerID,
		"session_id": c.cfg.SessionID,
		"epoch":      c.epoch,
		"events":     events,
	}

	_, err := c.postJSON(ctx, "/workers/events/stream", body)
	if err != nil {
		// Re-queue events on failure
		c.mu.Lock()
		c.eventBuf = append(events, c.eventBuf...)
		if len(c.eventBuf) > MaxStreamEventBufferSize {
			c.eventBuf = c.eventBuf[:MaxStreamEventBufferSize]
		}
		c.mu.Unlock()
		return err
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, path string, body interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			delay := RetryBaseDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.cfg.BaseURL+path, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if c.cfg.AuthToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var result map[string]interface{}
			if len(respBody) > 0 {
				if err := json.Unmarshal(respBody, &result); err != nil {
					return nil, err
				}
			}
			return result, nil
		}

		lastErr = fmt.Errorf("CCR %s returned %d: %s", path, resp.StatusCode, string(respBody))

		// Don't retry on 4xx (except 429)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return nil, lastErr
		}
	}

	return nil, fmt.Errorf("CCR %s failed after %d retries: %w", path, MaxRetries, lastErr)
}
