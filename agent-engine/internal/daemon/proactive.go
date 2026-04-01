package daemon

import (
	"context"
	"log/slog"
	"time"
)

// ProactiveConfig configures the proactive mode ticker.
type ProactiveConfig struct {
	// Interval is how often the proactive callback fires.
	Interval time.Duration
	// OnTick is called on every tick with the current tick count.
	OnTick func(ctx context.Context, tick int)
}

// ProactiveMode drives periodic background actions (the Go equivalent of the
// KAIROS proactive-mode loop).  It runs until ctx is cancelled.
type ProactiveMode struct {
	cfg ProactiveConfig
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

	tick := 0
	slog.Info("proactive mode started", slog.Duration("interval", p.cfg.Interval))

	for {
		select {
		case <-ctx.Done():
			slog.Info("proactive mode: context cancelled, stopping")
			return

		case <-ticker.C:
			tick++
			slog.Debug("proactive tick", slog.Int("tick", tick))
			if p.cfg.OnTick != nil {
				p.cfg.OnTick(ctx, tick)
			}
		}
	}
}
