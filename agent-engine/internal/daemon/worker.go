package daemon

import (
	"context"
	"log/slog"
	"time"
)

const heartbeatInterval = 30 * time.Second

// Worker executes scheduled tasks inside the daemon process and emits
// periodic heartbeats so the supervisor knows it is alive.
type Worker struct {
	scheduler *Scheduler
	onTask    func(ctx context.Context, task *ScheduledTask) error
}

// NewWorker creates a Worker backed by the given Scheduler.
// onTask is called each time the scheduler fires a task.
func NewWorker(scheduler *Scheduler, onTask func(context.Context, *ScheduledTask) error) *Worker {
	return &Worker{scheduler: scheduler, onTask: onTask}
}

// Run starts the heartbeat ticker and the scheduler loop, blocking until ctx
// is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// Wire onTask into the scheduler.
	w.scheduler.onRun = func(task *ScheduledTask) {
		if w.onTask == nil {
			return
		}
		taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		slog.Info("worker: running task", slog.String("id", task.ID))
		if err := w.onTask(taskCtx, task); err != nil {
			slog.Warn("worker: task error",
				slog.String("id", task.ID),
				slog.Any("err", err))
		}
	}

	go func() {
		if err := w.scheduler.Start(ctx); err != nil && err != context.Canceled {
			slog.Error("worker: scheduler error", slog.Any("err", err))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-ticker.C:
			slog.Debug("worker: heartbeat", slog.String("at", t.UTC().Format(time.RFC3339)))
		}
	}
}
