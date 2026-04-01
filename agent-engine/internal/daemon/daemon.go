package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/wall-ai/agent-engine/internal/util"
)

const (
	daemonName    = "agent-engine-daemon"
	startupDelay  = 500 * time.Millisecond
	watchDebounce = 300 * time.Millisecond
)

// Config holds daemon startup options.
type Config struct {
	PIDFile   string
	WatchDirs []string
	// OnFileChange is called when a watched file changes.
	OnFileChange func(path string)
}

// Daemon is the long-running background process (equivalent of KAIROS).
type Daemon struct {
	cfg     Config
	watcher *fsnotify.Watcher
}

// New creates a Daemon.
func New(cfg Config) (*Daemon, error) {
	if cfg.PIDFile == "" {
		cfg.PIDFile = util.PIDFilePath(daemonName)
	}
	return &Daemon{cfg: cfg}, nil
}

// Start launches the daemon: writes the PID file, sets up file watching,
// and runs the event loop until ctx is cancelled.
func (d *Daemon) Start(ctx context.Context) error {
	// Single-instance guard via PID file.
	if err := d.acquirePID(); err != nil {
		return err
	}
	defer util.RemovePIDFile(d.cfg.PIDFile)

	util.RegisterCleanup(func() {
		slog.Info("daemon cleanup: removing PID file")
		_ = util.RemovePIDFile(d.cfg.PIDFile)
	})

	// Set up fsnotify watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	d.watcher = watcher
	defer watcher.Close()

	for _, dir := range d.cfg.WatchDirs {
		if err := watcher.Add(dir); err != nil {
			slog.Warn("daemon: cannot watch dir", slog.String("dir", dir), slog.Any("err", err))
		}
	}

	slog.Info("daemon started", slog.Int("pid", os.Getpid()))

	// Debounce map: path → timer
	debounce := make(map[string]*time.Timer)

	for {
		select {
		case <-ctx.Done():
			slog.Info("daemon: context cancelled, shutting down")
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				path := event.Name
				if t, exists := debounce[path]; exists {
					t.Stop()
				}
				debounce[path] = time.AfterFunc(watchDebounce, func() {
					if d.cfg.OnFileChange != nil {
						d.cfg.OnFileChange(path)
					}
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("daemon watcher error", slog.Any("err", err))
		}
	}
}

// IsRunning reports whether a daemon with the configured PID file is alive.
func (d *Daemon) IsRunning() bool {
	pid, err := util.ReadPIDFile(d.cfg.PIDFile)
	if err != nil {
		return false
	}
	return util.IsProcessAlive(pid)
}

// acquirePID writes the current PID to the PID file, failing if another
// instance is already running.
func (d *Daemon) acquirePID() error {
	if err := util.EnsureDir(util.DefaultPIDDir()); err != nil {
		return err
	}

	// Check if already running.
	if util.PIDFileExists(d.cfg.PIDFile) {
		pid, err := util.ReadPIDFile(d.cfg.PIDFile)
		if err == nil && util.IsProcessAlive(pid) {
			return fmt.Errorf("daemon already running with PID %d", pid)
		}
		// Stale PID file — remove and proceed.
		_ = util.RemovePIDFile(d.cfg.PIDFile)
	}

	return util.WritePIDFile(d.cfg.PIDFile)
}
