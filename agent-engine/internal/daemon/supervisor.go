package daemon

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

const (
	maxRestarts    = 5
	restartBackoff = 2 * time.Second
)

// Supervisor spawns and health-checks a child process, restarting it on failure.
type Supervisor struct {
	binaryPath string
	args       []string
	cmd        *exec.Cmd
	restarts   int
}

// NewSupervisor creates a Supervisor that manages the given binary.
func NewSupervisor(binaryPath string, args ...string) *Supervisor {
	return &Supervisor{binaryPath: binaryPath, args: args}
}

// Run starts the child process and restarts it up to maxRestarts times if it
// exits unexpectedly.  It returns when ctx is cancelled or the process has
// exceeded the restart limit.
func (s *Supervisor) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		s.cmd = exec.CommandContext(ctx, s.binaryPath, s.args...)
		s.cmd.Stdout = os.Stdout
		s.cmd.Stderr = os.Stderr

		slog.Info("supervisor: starting process",
			slog.String("binary", s.binaryPath),
			slog.Int("restart", s.restarts))

		if err := s.cmd.Start(); err != nil {
			return err
		}

		done := make(chan error, 1)
		go func() { done <- s.cmd.Wait() }()

		select {
		case <-ctx.Done():
			_ = s.cmd.Process.Kill()
			return ctx.Err()

		case err := <-done:
			if err == nil {
				slog.Info("supervisor: process exited cleanly")
				return nil
			}
			s.restarts++
			slog.Warn("supervisor: process exited with error",
				slog.Any("err", err),
				slog.Int("restarts", s.restarts))

			if s.restarts >= maxRestarts {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(restartBackoff * time.Duration(s.restarts)):
			}
		}
	}
}
