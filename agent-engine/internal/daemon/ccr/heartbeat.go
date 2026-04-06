package ccr

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Heartbeat manages the periodic heartbeat goroutine for a CCR worker.
// Aligned with claude-code-main ccrClient.ts heartbeat logic.
type Heartbeat struct {
	client   *Client
	interval time.Duration
	mu       sync.Mutex
	state    WorkerState
	cancel   context.CancelFunc
}

// NewHeartbeat creates a Heartbeat tied to the given CCR client.
func NewHeartbeat(client *Client, interval time.Duration) *Heartbeat {
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}
	return &Heartbeat{
		client:   client,
		interval: interval,
		state:    WorkerStateIdle,
	}
}

// SetState updates the reported worker state.
func (h *Heartbeat) SetState(state WorkerState) {
	h.mu.Lock()
	h.state = state
	h.mu.Unlock()
}

// State returns the current worker state.
func (h *Heartbeat) State() WorkerState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

// Start begins the heartbeat loop. It blocks until ctx is cancelled or Stop is called.
func (h *Heartbeat) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	h.cancel = cancel

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.mu.Lock()
			state := h.state
			h.mu.Unlock()

			h.client.WriteEvent("heartbeat", map[string]interface{}{
				"worker_state": string(state),
				"timestamp":    time.Now().UnixMilli(),
			})

			slog.Debug("ccr heartbeat: sent",
				slog.String("state", string(state)))
		}
	}
}

// Stop cancels the heartbeat loop.
func (h *Heartbeat) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
}
