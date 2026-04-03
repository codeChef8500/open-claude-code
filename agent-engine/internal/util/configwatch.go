package util

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// ConfigWatcher — watches config files for changes and triggers reloads.
// Aligned with claude-code-main's config hot-reload pattern.
// ────────────────────────────────────────────────────────────────────────────

// ConfigChangeHandler is called when a watched config file changes.
type ConfigChangeHandler func(path string)

// ConfigWatcher polls config files for modification and triggers handlers.
type ConfigWatcher struct {
	mu       sync.Mutex
	watches  map[string]watchEntry
	handlers []ConfigChangeHandler
	interval time.Duration
	cancel   context.CancelFunc
}

type watchEntry struct {
	path    string
	modTime time.Time
	size    int64
}

// NewConfigWatcher creates a watcher with the given poll interval.
func NewConfigWatcher(interval time.Duration) *ConfigWatcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &ConfigWatcher{
		watches:  make(map[string]watchEntry),
		interval: interval,
	}
}

// Watch adds a file path to the watch list.
func (cw *ConfigWatcher) Watch(path string) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		cw.watches[path] = watchEntry{path: path}
		return
	}
	cw.watches[path] = watchEntry{
		path:    path,
		modTime: info.ModTime(),
		size:    info.Size(),
	}
}

// OnChange registers a handler that fires when any watched file changes.
func (cw *ConfigWatcher) OnChange(handler ConfigChangeHandler) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.handlers = append(cw.handlers, handler)
}

// Start begins polling in a background goroutine.
func (cw *ConfigWatcher) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	cw.cancel = cancel

	go func() {
		ticker := time.NewTicker(cw.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cw.checkAll()
			}
		}
	}()
}

// Stop halts the polling goroutine.
func (cw *ConfigWatcher) Stop() {
	if cw.cancel != nil {
		cw.cancel()
	}
}

func (cw *ConfigWatcher) checkAll() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	for path, entry := range cw.watches {
		info, err := os.Stat(path)
		if err != nil {
			// File might have been deleted — if it existed before, notify.
			if !entry.modTime.IsZero() {
				cw.watches[path] = watchEntry{path: path}
				cw.notifyLocked(path)
			}
			continue
		}

		if info.ModTime() != entry.modTime || info.Size() != entry.size {
			cw.watches[path] = watchEntry{
				path:    path,
				modTime: info.ModTime(),
				size:    info.Size(),
			}
			cw.notifyLocked(path)
		}
	}
}

func (cw *ConfigWatcher) notifyLocked(path string) {
	for _, h := range cw.handlers {
		go func(handler ConfigChangeHandler) {
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("config watcher handler panicked", "path", path, "panic", r)
				}
			}()
			handler(path)
		}(h)
	}
}
