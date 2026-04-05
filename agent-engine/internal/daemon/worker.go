package daemon

import (
	"context"
	"log/slog"
	"time"
)

const heartbeatInterval = 30 * time.Second

// WorkerKind identifies the type of daemon worker.
type WorkerKind string

const (
	// WorkerKindAssistant is the KAIROS assistant daemon worker.
	WorkerKindAssistant WorkerKind = "assistant"
)

// WorkerConfig configures a daemon Worker instance.
// Aligned with claude-code-main daemon/workerRegistry.js.
type WorkerConfig struct {
	// Kind identifies this worker type.
	Kind WorkerKind
	// Epoch is the monotonically increasing epoch (CCR uses this for supersession).
	Epoch int
	// SessionKind is the session kind to register in the PID file.
	SessionKind string // typically "daemon-worker"
	// ProjectDir is the project root this worker operates on.
	ProjectDir string
	// OnTask is called when the scheduler fires a task.
	OnTask func(ctx context.Context, task *CronTask) error
	// OnTick is called on every proactive-mode tick.
	OnTick func(ctx context.Context, tick int)
	// OnUserMessage is called when a user message arrives via IPC.
	OnUserMessage func(ctx context.Context, msg string) error
}

// Worker executes scheduled tasks inside the daemon process and emits
// periodic heartbeats so the supervisor knows it is alive.
// Aligned with claude-code-main daemon worker process.
type Worker struct {
	cfg       WorkerConfig
	scheduler *CronScheduler
}

// NewWorker creates a Worker backed by the given CronScheduler.
func NewWorker(cfg WorkerConfig, scheduler *CronScheduler) *Worker {
	return &Worker{cfg: cfg, scheduler: scheduler}
}

// Run starts the heartbeat ticker and the scheduler loop, blocking until ctx
// is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// Start the scheduler in the background.
	go func() {
		if err := w.scheduler.Start(ctx); err != nil {
			slog.Error("worker: scheduler error", slog.Any("err", err))
		}
	}()

	slog.Info("worker: started",
		slog.String("kind", string(w.cfg.Kind)),
		slog.Int("epoch", w.cfg.Epoch))

	for {
		select {
		case <-ctx.Done():
			w.scheduler.Stop()
			return ctx.Err()
		case t := <-ticker.C:
			slog.Debug("worker: heartbeat", slog.String("at", t.UTC().Format(time.RFC3339)))
		}
	}
}
