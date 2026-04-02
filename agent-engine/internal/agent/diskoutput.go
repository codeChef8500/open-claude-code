package agent

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
)

const (
	// maxTaskOutputBytes is the 5 GB hard cap on per-task disk output.
	maxTaskOutputBytes int64 = 5 * 1024 * 1024 * 1024
)

// DiskOutput serialises writes for a single task's output file through a
// single drain goroutine, avoiding Promise-chain memory build-up.
//
// Architecture:
//
//	Write() → queue (chan) → drain goroutine → os.File
//
// The drain goroutine stops when Close() is called.
type DiskOutput struct {
	path     string
	queue    chan string
	done     chan struct{}
	written  atomic.Int64
	sessionID string
}

// NewDiskOutput creates a DiskOutput that writes to path.
// The file is opened with O_NOFOLLOW to prevent symlink attacks.
// sessionID is embedded in the path to avoid concurrent-session conflicts.
func NewDiskOutput(dir, agentID, sessionID string) (*DiskOutput, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("disk output: mkdir %s: %w", dir, err)
	}

	filename := fmt.Sprintf("%s-%s.log", sessionID, agentID)
	path := filepath.Join(dir, filename)

	d := &DiskOutput{
		path:      path,
		queue:     make(chan string, 256),
		done:      make(chan struct{}),
		sessionID: sessionID,
	}
	go d.drain()
	return d, nil
}

// Write enqueues a string for asynchronous disk write.
// Returns false if the output has been closed or the size cap is reached.
func (d *DiskOutput) Write(s string) bool {
	if d.written.Load() >= maxTaskOutputBytes {
		return false
	}
	select {
	case <-d.done:
		return false
	case d.queue <- s:
		return true
	default:
		// Queue full — drop rather than block the caller.
		slog.Debug("disk output: queue full, dropping chunk", slog.String("path", d.path))
		return false
	}
}

// Path returns the absolute path of the output file.
func (d *DiskOutput) Path() string { return d.path }

// BytesWritten returns the total bytes flushed to disk so far.
func (d *DiskOutput) BytesWritten() int64 { return d.written.Load() }

// Close flushes the queue and stops the drain goroutine.
func (d *DiskOutput) Close() {
	select {
	case <-d.done:
	default:
		close(d.done)
	}
}

// drain is the single-goroutine writer.
func (d *DiskOutput) drain() {
	// Open with O_NOFOLLOW to prevent symlink attacks.
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	// O_NOFOLLOW is available on Linux/macOS but not Windows.
	// We call a platform-specific helper; see diskoutput_unix.go / diskoutput_windows.go.
	f, err := openNoFollow(d.path, flags, 0o600)
	if err != nil {
		slog.Warn("disk output: open failed", slog.String("path", d.path), slog.Any("err", err))
		return
	}
	defer f.Close()

	for {
		select {
		case s, ok := <-d.queue:
			if !ok {
				return
			}
			if d.written.Load()+int64(len(s)) > maxTaskOutputBytes {
				slog.Warn("disk output: 5 GB cap reached, dropping remaining output", slog.String("path", d.path))
				return
			}
			n, err := f.WriteString(s)
			if err != nil {
				slog.Warn("disk output: write error", slog.String("path", d.path), slog.Any("err", err))
				continue
			}
			d.written.Add(int64(n))
		case <-d.done:
			// Drain remaining items before exiting.
			for {
				select {
				case s := <-d.queue:
					_, _ = f.WriteString(s)
				default:
					return
				}
			}
		}
	}
}
