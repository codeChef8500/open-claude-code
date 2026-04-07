package teammem

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Team memory watcher — periodic background sync
// ────────────────────────────────────────────────────────────────────────────

const (
	// DefaultSyncInterval is the default interval between background syncs.
	DefaultSyncInterval = 5 * time.Minute
	// MinSyncInterval is the minimum allowed sync interval.
	MinSyncInterval = 30 * time.Second
)

// Watcher runs periodic team memory synchronization in the background.
type Watcher struct {
	syncService  *SyncService
	state        *SyncState
	interval     time.Duration
	cancel       context.CancelFunc
	done         chan struct{}
	mu           sync.Mutex
	running      bool
	lastSyncTime time.Time
	lastResult   *SyncResult
}

// WatcherOption configures a Watcher.
type WatcherOption func(*Watcher)

// WithSyncInterval sets the sync interval.
func WithSyncInterval(d time.Duration) WatcherOption {
	return func(w *Watcher) {
		if d >= MinSyncInterval {
			w.interval = d
		}
	}
}

// NewWatcher creates a new team memory watcher.
func NewWatcher(syncService *SyncService, state *SyncState, opts ...WatcherOption) *Watcher {
	w := &Watcher{
		syncService: syncService,
		state:       state,
		interval:    DefaultSyncInterval,
		done:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Start begins periodic background synchronization.
// Returns immediately. Call Stop() to shut down.
func (w *Watcher) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.running = true

	go w.loop(ctx)
}

// Stop gracefully shuts down the watcher.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancel()
	<-w.done
	w.running = false
}

// IsRunning reports whether the watcher is active.
func (w *Watcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// LastSync returns the time and result of the last sync.
func (w *Watcher) LastSync() (time.Time, *SyncResult) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastSyncTime, w.lastResult
}

// SyncNow triggers an immediate sync outside the regular interval.
func (w *Watcher) SyncNow(ctx context.Context) (*SyncResult, error) {
	return w.doSync(ctx)
}

func (w *Watcher) loop(ctx context.Context) {
	defer close(w.done)

	// Initial sync on start
	if _, err := w.doSync(ctx); err != nil {
		slog.Warn("team memory watcher: initial sync failed", slog.Any("err", err))
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := w.doSync(ctx); err != nil {
				slog.Warn("team memory watcher: periodic sync failed", slog.Any("err", err))
			}
		}
	}
}

func (w *Watcher) doSync(ctx context.Context) (*SyncResult, error) {
	result, err := w.syncService.Sync(ctx, w.state)

	w.mu.Lock()
	w.lastSyncTime = time.Now()
	w.lastResult = result
	w.mu.Unlock()

	if err != nil {
		return result, err
	}

	if result != nil && (result.PulledCount > 0 || result.PushedCount > 0) {
		slog.Info("team memory sync completed",
			slog.Int("pulled", result.PulledCount),
			slog.Int("pushed", result.PushedCount),
			slog.Int("skipped", len(result.SkippedFiles)))
	}

	return result, nil
}
