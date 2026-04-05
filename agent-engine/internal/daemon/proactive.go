package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// ProactiveConfig configures the proactive mode ticker.
// Aligned with KAIROS tick injection mechanism.
type ProactiveConfig struct {
	// Interval is how often the proactive callback fires (default 1 min).
	Interval time.Duration
	// OnTick is called on every tick with the current tick count.
	OnTick func(ctx context.Context, tick int)
	// IsIdle returns true if the REPL is idle (not mid-query).
	// Ticks are only injected when idle. If nil, always fires.
	IsIdle func() bool
	// OnTickMessage is called with the formatted XML tick message for injection
	// into the conversation. If nil, only OnTick is called.
	OnTickMessage func(ctx context.Context, msg string)
}

// ProactiveMode drives periodic background actions (the Go equivalent of the
// KAIROS proactive-mode loop). It injects <tick> messages into the conversation
// when idle, allowing the model to decide if there is work to do.
type ProactiveMode struct {
	cfg     ProactiveConfig
	paused  atomic.Bool
	tickNum atomic.Int64
}

// NewProactiveMode creates a ProactiveMode with the given config.
// If Interval is zero it defaults to 1 minute.
func NewProactiveMode(cfg ProactiveConfig) *ProactiveMode {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Minute
	}
	return &ProactiveMode{cfg: cfg}
}

// Run starts the ticker loop and blocks until ctx is cancelled.
func (p *ProactiveMode) Run(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	slog.Info("proactive mode started", slog.Duration("interval", p.cfg.Interval))

	for {
		select {
		case <-ctx.Done():
			slog.Info("proactive mode: context cancelled, stopping")
			return

		case <-ticker.C:
			if p.paused.Load() {
				continue
			}

			// Only fire when idle (not mid-query)
			if p.cfg.IsIdle != nil && !p.cfg.IsIdle() {
				slog.Debug("proactive tick: skipped (not idle)")
				continue
			}

			tick := int(p.tickNum.Add(1))
			slog.Debug("proactive tick", slog.Int("tick", tick))

			if p.cfg.OnTick != nil {
				p.cfg.OnTick(ctx, tick)
			}

			if p.cfg.OnTickMessage != nil {
				msg := FormatTickMessage(tick, time.Now())
				p.cfg.OnTickMessage(ctx, msg)
			}
		}
	}
}

// Pause temporarily stops tick emission without stopping the goroutine.
func (p *ProactiveMode) Pause() {
	p.paused.Store(true)
}

// Resume resumes tick emission after a Pause.
func (p *ProactiveMode) Resume() {
	p.paused.Store(false)
}

// TickCount returns the current tick number.
func (p *ProactiveMode) TickCount() int {
	return int(p.tickNum.Load())
}

// FormatTickMessage creates the XML tick tag injected into the conversation.
// Format: <tick count="N" timestamp="2006-01-02T15:04:05Z07:00"/>
func FormatTickMessage(count int, ts time.Time) string {
	return fmt.Sprintf(`<tick count="%d" timestamp="%s"/>`, count, ts.Format(time.RFC3339))
}
