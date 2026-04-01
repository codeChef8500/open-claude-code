package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wall-ai/agent-engine/internal/util"
)

const (
	lockAcquireTimeout = 5 * time.Second
	lockRetryInterval  = 50 * time.Millisecond
)

// SchedulerLock is an advisory file lock that prevents concurrent schedulers
// from running.  It uses O_EXCL atomic creation so that only one process can
// hold the lock at a time.  Stale locks (left by crashed processes) are
// detected via PID liveness checks and automatically recovered.
type SchedulerLock struct {
	lockFile string
}

// NewSchedulerLock creates a SchedulerLock for the given service name.
func NewSchedulerLock(serviceName string) *SchedulerLock {
	lockDir := util.DefaultPIDDir()
	return &SchedulerLock{
		lockFile: filepath.Join(lockDir, serviceName+".lock"),
	}
}

// Acquire tries to obtain the lock, retrying until lockAcquireTimeout elapses.
// Returns an error if the lock cannot be acquired within the timeout.
func (l *SchedulerLock) Acquire() error {
	if err := util.EnsureDir(util.DefaultPIDDir()); err != nil {
		return fmt.Errorf("scheduler lock: ensure dir: %w", err)
	}

	deadline := time.Now().Add(lockAcquireTimeout)
	for time.Now().Before(deadline) {
		if err := l.tryAcquire(); err == nil {
			return nil
		}
		// Check for stale lock.
		if l.isStale() {
			_ = os.Remove(l.lockFile)
			continue
		}
		time.Sleep(lockRetryInterval)
	}
	return fmt.Errorf("scheduler lock: could not acquire %s within %s", l.lockFile, lockAcquireTimeout)
}

// Release removes the lock file, allowing other processes to acquire it.
func (l *SchedulerLock) Release() error {
	err := os.Remove(l.lockFile)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// tryAcquire attempts a single O_EXCL create of the lock file, writing the
// current PID so liveness can be checked later.
func (l *SchedulerLock) tryAcquire() error {
	f, err := os.OpenFile(l.lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%d\n", os.Getpid())
	return err
}

// isStale reports whether the lock file contains a PID for a dead process.
func (l *SchedulerLock) isStale() bool {
	pid, err := util.ReadPIDFile(l.lockFile)
	if err != nil {
		return true
	}
	return !util.IsProcessAlive(pid)
}
