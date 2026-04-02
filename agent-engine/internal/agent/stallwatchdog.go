package agent

import (
	"context"
	"log/slog"
	"os"
	"time"
)

const (
	stallCheckInterval = 5 * time.Second
	stallThreshold     = 45 * time.Second
)

// StallWatchdog monitors a DiskOutput file for output stalls.
// It polls the file size every stallCheckInterval; if the file has not grown
// for stallThreshold it fires the provided notification callback.
type StallWatchdog struct {
	output   *DiskOutput
	notify   func(agentID, message string)
	agentID  string
}

// NewStallWatchdog creates a StallWatchdog for the given DiskOutput.
// notify is called with (agentID, message) when a stall is detected.
func NewStallWatchdog(output *DiskOutput, agentID string, notify func(agentID, message string)) *StallWatchdog {
	return &StallWatchdog{
		output:  output,
		notify:  notify,
		agentID: agentID,
	}
}

// Run starts the watchdog loop.  It blocks until ctx is cancelled.
// Call it in a goroutine.
func (w *StallWatchdog) Run(ctx context.Context) {
	ticker := time.NewTicker(stallCheckInterval)
	defer ticker.Stop()

	var lastSize int64 = -1
	var lastGrowth time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			size := w.currentSize()
			now := time.Now()

			if size != lastSize {
				lastSize = size
				lastGrowth = now
				continue
			}

			// Size unchanged — check if we've exceeded the stall threshold.
			if !lastGrowth.IsZero() && now.Sub(lastGrowth) >= stallThreshold {
				slog.Debug("stall watchdog: output stalled",
					slog.String("agent", w.agentID),
					slog.Duration("stalled_for", now.Sub(lastGrowth)))
				if w.notify != nil {
					w.notify(w.agentID,
						"Agent output has not changed for "+stallThreshold.String()+". The task may be waiting for input.")
				}
				// Reset so we don't re-fire every tick.
				lastGrowth = now
			}
		}
	}
}

func (w *StallWatchdog) currentSize() int64 {
	info, err := os.Stat(w.output.Path())
	if err != nil {
		return -1
	}
	return info.Size()
}
