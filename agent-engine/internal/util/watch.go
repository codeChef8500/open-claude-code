package util

import (
	"context"
	"os"
	"time"
)

// FileWatcher polls a file for changes and notifies via a channel.
// This is a simple polling watcher suitable for config files and CLAUDE.md.
type FileWatcher struct {
	path     string
	interval time.Duration
	lastMod  time.Time
	lastSize int64
}

// NewFileWatcher creates a watcher for path with the given poll interval.
// If interval is 0, it defaults to 2 seconds.
func NewFileWatcher(path string, interval time.Duration) *FileWatcher {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &FileWatcher{path: path, interval: interval}
}

// Watch starts polling and sends the path to ch whenever the file changes.
// Returns when ctx is cancelled. The channel is not closed by Watch.
func (w *FileWatcher) Watch(ctx context.Context, ch chan<- string) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if w.changed() {
				select {
				case ch <- w.path:
				default:
				}
			}
		}
	}
}

func (w *FileWatcher) changed() bool {
	info, err := os.Stat(w.path)
	if err != nil {
		return false
	}
	if info.ModTime() != w.lastMod || info.Size() != w.lastSize {
		w.lastMod = info.ModTime()
		w.lastSize = info.Size()
		return true
	}
	return false
}

// DirWatcher polls a directory for file additions/removals.
type DirWatcher struct {
	dir      string
	interval time.Duration
	snapshot map[string]time.Time
}

// NewDirWatcher creates a watcher for the given directory.
func NewDirWatcher(dir string, interval time.Duration) *DirWatcher {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return &DirWatcher{
		dir:      dir,
		interval: interval,
		snapshot: make(map[string]time.Time),
	}
}

// DirChangeEvent describes a change inside a watched directory.
type DirChangeEvent struct {
	Path   string
	Change DirChange
}

// DirChange classifies the type of directory change.
type DirChange int

const (
	DirChangeAdded   DirChange = 1
	DirChangeRemoved DirChange = 2
	DirChangeModified DirChange = 3
)

// Watch polls the directory and emits change events.
func (d *DirWatcher) Watch(ctx context.Context, ch chan<- DirChangeEvent) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.diff(ch)
		}
	}
}

func (d *DirWatcher) diff(ch chan<- DirChangeEvent) {
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return
	}
	current := make(map[string]time.Time, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		path := d.dir + "/" + e.Name()
		current[path] = info.ModTime()
		prev, seen := d.snapshot[path]
		if !seen {
			select {
			case ch <- DirChangeEvent{Path: path, Change: DirChangeAdded}:
			default:
			}
		} else if info.ModTime() != prev {
			select {
			case ch <- DirChangeEvent{Path: path, Change: DirChangeModified}:
			default:
			}
		}
	}
	for path := range d.snapshot {
		if _, ok := current[path]; !ok {
			select {
			case ch <- DirChangeEvent{Path: path, Change: DirChangeRemoved}:
			default:
			}
		}
	}
	d.snapshot = current
}
