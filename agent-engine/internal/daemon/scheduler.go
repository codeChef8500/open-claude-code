package daemon

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ─── Scheduler Mode ─────────────────────────────────────────────────────────

// SchedulerMode determines the scheduler's task-filtering behavior.
type SchedulerMode string

const (
	// SchedulerModeREPL runs all tasks (session + durable).
	SchedulerModeREPL SchedulerMode = "repl"
	// SchedulerModeDaemon runs only permanent durable tasks.
	SchedulerModeDaemon SchedulerMode = "daemon"
)

// ─── Scheduler Options ──────────────────────────────────────────────────────

const (
	schedulerCheckInterval     = 1 * time.Second
	schedulerLockProbeInterval = 5 * time.Second
	schedulerFileStabilityMs   = 300 * time.Millisecond
)

// SchedulerOptions configures a CronScheduler instance.
// Aligned with claude-code-main utils/cronScheduler.ts createCronScheduler.
type SchedulerOptions struct {
	// Mode controls which tasks are scheduled (repl = all, daemon = permanent only).
	Mode SchedulerMode
	// AssistantMode is true when running under KAIROS assistant mode.
	AssistantMode bool
	// ProjectDir is the project root where .claude/ lives.
	ProjectDir string
	// LockIdentity is the identity written into the lock file.
	// Daemon uses a stable UUID; REPL uses current PID string.
	LockIdentity string
	// Store is the task store to read tasks from.
	Store *CronTaskStore
	// JitterConfig configures task jitter. Uses DefaultJitterConfig if nil.
	JitterCfg *JitterConfig
	// OnFire is called when a task should be executed.
	OnFire func(task *CronTask)
	// OnMissed is called with tasks that were missed while the process was down.
	OnMissed func(tasks []*CronTask)
	// IsKilled is a kill-switch polling function. Returns true to stop the scheduler.
	IsKilled func() bool
}

// ─── CronScheduler ──────────────────────────────────────────────────────────

// CronScheduler manages scheduled tasks with process-safe locking,
// active/passive mode switching, file watching, and jitter.
// Aligned with claude-code-main utils/cronScheduler.ts.
type CronScheduler struct {
	opts   SchedulerOptions
	jitter JitterConfig
	lock   *SchedulerLock

	mu       sync.Mutex
	active   bool // true if we hold the scheduler lock
	stopCh   chan struct{}
	stopped  bool
	watcher  *fsnotify.Watcher
	lastFile time.Time // last known mtime of scheduled_tasks.json
}

// NewCronScheduler creates a CronScheduler with the given options.
func NewCronScheduler(opts SchedulerOptions) *CronScheduler {
	jitter := DefaultJitterConfig
	if opts.JitterCfg != nil {
		jitter = *opts.JitterCfg
	}
	lockName := "cron-scheduler"
	if opts.ProjectDir != "" {
		lockName = "cron-" + sanitizeLockName(opts.ProjectDir)
	}
	return &CronScheduler{
		opts:   opts,
		jitter: jitter,
		lock:   NewSchedulerLock(lockName),
		stopCh: make(chan struct{}),
	}
}

// Start runs the scheduler event loop until ctx is cancelled or Stop is called.
// It attempts to acquire the scheduler lock to become active. If another process
// holds the lock, it runs in passive mode and periodically probes for lock recovery.
func (cs *CronScheduler) Start(ctx context.Context) error {
	// Try to become active
	cs.tryBecomeActive()

	// Handle missed tasks on startup (durable only)
	if cs.opts.Store != nil && cs.opts.OnMissed != nil {
		missed := cs.opts.Store.FindMissed(time.Now().UnixMilli())
		if len(missed) > 0 {
			slog.Info("scheduler: found missed tasks", slog.Int("count", len(missed)))
			cs.opts.OnMissed(missed)
		}
	}

	// Expire aged recurring tasks
	if cs.opts.Store != nil {
		cs.opts.Store.RemoveExpired(cs.jitter.RecurringMaxAgeMs)
	}

	// Set up file watcher for scheduled_tasks.json changes
	cs.startFileWatcher()

	checkTicker := time.NewTicker(schedulerCheckInterval)
	defer checkTicker.Stop()

	probeTicker := time.NewTicker(schedulerLockProbeInterval)
	defer probeTicker.Stop()

	defer cs.cleanup()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-cs.stopCh:
			return nil

		case <-checkTicker.C:
			// Kill switch check
			if cs.opts.IsKilled != nil && cs.opts.IsKilled() {
				slog.Info("scheduler: killed by kill switch")
				return nil
			}
			if cs.isActive() {
				cs.check()
			}

		case <-probeTicker.C:
			if !cs.isActive() {
				cs.probePassive()
			}
		}
	}
}

// Stop signals the scheduler to stop.
func (cs *CronScheduler) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if !cs.stopped {
		cs.stopped = true
		close(cs.stopCh)
	}
}

// IsActive returns whether this scheduler instance holds the lock.
func (cs *CronScheduler) IsActive() bool {
	return cs.isActive()
}

// ─── internal ───────────────────────────────────────────────────────────────

func (cs *CronScheduler) isActive() bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.active
}

func (cs *CronScheduler) tryBecomeActive() {
	if err := cs.lock.Acquire(); err != nil {
		slog.Debug("scheduler: running in passive mode", slog.Any("err", err))
		return
	}
	cs.mu.Lock()
	cs.active = true
	cs.mu.Unlock()
	slog.Info("scheduler: acquired lock, active mode")
}

// probePassive checks if the lock holder is still alive. If it crashed,
// we recover the lock and become active.
func (cs *CronScheduler) probePassive() {
	if cs.lock.isStale() {
		slog.Info("scheduler: lock holder dead, recovering")
		_ = cs.lock.Release()
		cs.tryBecomeActive()
	}
}

// check iterates through tasks and fires any that are due.
func (cs *CronScheduler) check() {
	if cs.opts.Store == nil || cs.opts.OnFire == nil {
		return
	}

	nowMs := time.Now().UnixMilli()
	tasks := cs.tasksForMode()

	for _, task := range tasks {
		baseMs := task.LastFiredAt
		if baseMs == 0 {
			baseMs = task.CreatedAt
		}

		var nextMs int64
		if task.Recurring {
			nextMs = JitteredNextCronRunMs(task.Cron, baseMs, cs.jitter)
		} else {
			nextMs = OneShotJitteredNextCronRunMs(task.Cron, baseMs, cs.jitter)
		}

		if nextMs > 0 && nextMs <= nowMs {
			slog.Info("scheduler: firing task",
				slog.String("id", task.ID),
				slog.Bool("recurring", task.Recurring))

			cs.opts.Store.MarkFired(task.ID)
			cs.opts.OnFire(task)

			// Remove one-shot tasks after firing
			if !task.Recurring {
				_ = cs.opts.Store.Remove([]string{task.ID})
			}
		}
	}
}

// tasksForMode returns the tasks appropriate for the current scheduler mode.
func (cs *CronScheduler) tasksForMode() []*CronTask {
	if cs.opts.Store == nil {
		return nil
	}
	allTasks := cs.opts.Store.ListAll()
	if cs.opts.Mode == SchedulerModeDaemon {
		// Daemon mode: only permanent durable tasks
		var filtered []*CronTask
		for _, t := range allTasks {
			if t.Permanent && t.Durable {
				filtered = append(filtered, t)
			}
		}
		return filtered
	}
	return allTasks
}

func (cs *CronScheduler) startFileWatcher() {
	if cs.opts.Store == nil {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("scheduler: cannot create file watcher", slog.Any("err", err))
		return
	}
	cs.watcher = watcher

	filePath := cs.opts.Store.FilePath()
	// Watch the directory containing the file (fsnotify requires directory watch)
	dir := filepath.Dir(filePath)
	if err := watcher.Add(dir); err != nil {
		slog.Debug("scheduler: cannot watch dir", slog.String("dir", dir), slog.Any("err", err))
		watcher.Close()
		cs.watcher = nil
		return
	}

	go func() {
		var debounceTimer *time.Timer
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Name == filePath && (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
					// Debounce file changes
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(schedulerFileStabilityMs, func() {
						slog.Debug("scheduler: task file changed, re-checking")
						if cs.isActive() {
							cs.check()
						}
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Warn("scheduler: watcher error", slog.Any("err", err))
			}
		}
	}()
}

func (cs *CronScheduler) cleanup() {
	if cs.watcher != nil {
		cs.watcher.Close()
	}
	if cs.isActive() {
		_ = cs.lock.Release()
	}
}

// sanitizeLockName creates a filesystem-safe name from a path.
func sanitizeLockName(s string) string {
	out := make([]byte, 0, len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			out = append(out, byte(c))
		} else {
			out = append(out, '_')
		}
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return string(out)
}
