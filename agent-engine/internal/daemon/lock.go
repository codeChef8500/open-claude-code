package daemon

import (
	"encoding/json"
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

// LockContent is the JSON structure written to the lock file.
// Extended from plain PID to support lockIdentity for daemon mode.
// Aligned with claude-code-main utils/cronTasksLock.ts.
type LockContent struct {
	PID        int    `json:"pid"`
	Identity   string `json:"identity"`
	AcquiredAt int64  `json:"acquired_at"`
}

// SchedulerLock is an advisory file lock that prevents concurrent schedulers
// from running.  It uses O_EXCL atomic creation so that only one process can
// hold the lock at a time.  Stale locks (left by crashed processes) are
// detected via PID liveness checks and automatically recovered.
//
// The lock file contains JSON: {pid, identity, acquired_at}.
// Identity is a stable UUID for daemon mode, or PID string for REPL mode.
type SchedulerLock struct {
	lockFile string
	identity string // stable identity for this lock holder
}

// NewSchedulerLock creates a SchedulerLock for the given service name.
// Uses PID as the default identity.
func NewSchedulerLock(serviceName string) *SchedulerLock {
	lockDir := util.DefaultPIDDir()
	return &SchedulerLock{
		lockFile: filepath.Join(lockDir, serviceName+".lock"),
		identity: fmt.Sprintf("pid-%d", os.Getpid()),
	}
}

// NewSchedulerLockWithIdentity creates a SchedulerLock with a stable identity.
// Daemon mode uses a UUID so the lock survives worker restarts within the same
// supervisor session.
func NewSchedulerLockWithIdentity(serviceName, identity string) *SchedulerLock {
	lockDir := util.DefaultPIDDir()
	return &SchedulerLock{
		lockFile: filepath.Join(lockDir, serviceName+".lock"),
		identity: identity,
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

// ReadLock reads the lock file and returns the lock content.
func (l *SchedulerLock) ReadLock() (*LockContent, error) {
	data, err := os.ReadFile(l.lockFile)
	if err != nil {
		return nil, err
	}
	var content LockContent
	if err := json.Unmarshal(data, &content); err != nil {
		// Fallback: try reading as plain PID (legacy format)
		pid, pidErr := util.ReadPIDFile(l.lockFile)
		if pidErr != nil {
			return nil, err
		}
		return &LockContent{PID: pid, Identity: fmt.Sprintf("pid-%d", pid)}, nil
	}
	return &content, nil
}

// tryAcquire attempts a single O_EXCL create of the lock file, writing
// JSON content with PID, identity, and acquisition time.
func (l *SchedulerLock) tryAcquire() error {
	f, err := os.OpenFile(l.lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	content := LockContent{
		PID:        os.Getpid(),
		Identity:   l.identity,
		AcquiredAt: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(content)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// isStale reports whether the lock file contains a PID for a dead process.
func (l *SchedulerLock) isStale() bool {
	content, err := l.ReadLock()
	if err != nil {
		return true
	}
	return !util.IsProcessAlive(content.PID)
}
