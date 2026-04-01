package daemon

import (
	"fmt"

	"github.com/wall-ai/agent-engine/internal/util"
)

// PIDRegistry manages PID file lifecycle for a named daemon service.
// It wraps the util-level PID helpers with single-instance enforcement.
type PIDRegistry struct {
	serviceName string
	pidFile     string
}

// NewPIDRegistry creates a PIDRegistry for the given service name.
// The PID file path is derived from the service name under the default PID dir.
func NewPIDRegistry(serviceName string) *PIDRegistry {
	return &PIDRegistry{
		serviceName: serviceName,
		pidFile:     util.PIDFilePath(serviceName),
	}
}

// PIDFile returns the absolute path of the PID file.
func (r *PIDRegistry) PIDFile() string { return r.pidFile }

// Acquire writes the current PID to the PID file, returning an error if
// another instance with the same service name is already running.
func (r *PIDRegistry) Acquire() error {
	if err := util.EnsureDir(util.DefaultPIDDir()); err != nil {
		return fmt.Errorf("pidregistry: ensure dir: %w", err)
	}

	if util.PIDFileExists(r.pidFile) {
		pid, err := util.ReadPIDFile(r.pidFile)
		if err == nil && util.IsProcessAlive(pid) {
			return fmt.Errorf("pidregistry: %s already running with PID %d", r.serviceName, pid)
		}
		// Stale PID file — clean up and continue.
		_ = util.RemovePIDFile(r.pidFile)
	}

	return util.WritePIDFile(r.pidFile)
}

// Release removes the PID file. Safe to call even if the file has been
// removed already (idempotent).
func (r *PIDRegistry) Release() error {
	return util.RemovePIDFile(r.pidFile)
}

// IsAlive reports whether the PID recorded in the PID file corresponds to a
// currently running process.
func (r *PIDRegistry) IsAlive() bool {
	pid, err := util.ReadPIDFile(r.pidFile)
	if err != nil {
		return false
	}
	return util.IsProcessAlive(pid)
}
