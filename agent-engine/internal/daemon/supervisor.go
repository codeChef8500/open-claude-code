package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	defaultMaxRestarts    = 5
	defaultRestartBackoff = 2 * time.Second
)

// SupervisorConfig configures a Supervisor instance.
// Aligned with claude-code-main daemon/main.js supervisor behavior.
type SupervisorConfig struct {
	// BinaryPath is the path to the agent-engine binary to spawn as workers.
	BinaryPath string
	// WorkerKinds lists the worker kinds to spawn and manage.
	WorkerKinds []WorkerKind
	// MaxRestarts is the max restart attempts per worker (default 5).
	MaxRestarts int
	// RestartBackoff is the base backoff duration between restarts (default 2s).
	RestartBackoff time.Duration
	// OnWorkerStart is called when a worker process starts.
	OnWorkerStart func(kind WorkerKind, pid int, epoch int)
	// OnWorkerStop is called when a worker process exits.
	OnWorkerStop func(kind WorkerKind, pid int, err error)
}

// workerState tracks a single managed worker process.
type workerState struct {
	kind     WorkerKind
	cmd      *exec.Cmd
	epoch    int
	restarts int
}

// Supervisor spawns and health-checks worker child processes, restarting
// them on failure. It manages multiple worker kinds with epoch tracking.
// Aligned with claude-code-main daemon supervisor process model.
type Supervisor struct {
	cfg     SupervisorConfig
	mu      sync.Mutex
	workers map[WorkerKind]*workerState
}

// NewSupervisor creates a Supervisor with the given config.
func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	if cfg.MaxRestarts <= 0 {
		cfg.MaxRestarts = defaultMaxRestarts
	}
	if cfg.RestartBackoff <= 0 {
		cfg.RestartBackoff = defaultRestartBackoff
	}
	return &Supervisor{
		cfg:     cfg,
		workers: make(map[WorkerKind]*workerState),
	}
}

// Run starts all configured worker kinds and blocks until ctx is cancelled
// or all workers exceed their restart limits.
func (s *Supervisor) Run(ctx context.Context) error {
	if len(s.cfg.WorkerKinds) == 0 {
		s.cfg.WorkerKinds = []WorkerKind{WorkerKindAssistant}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(s.cfg.WorkerKinds))

	for _, kind := range s.cfg.WorkerKinds {
		wg.Add(1)
		go func(k WorkerKind) {
			defer wg.Done()
			if err := s.runWorkerLoop(ctx, k); err != nil {
				errCh <- fmt.Errorf("worker %s: %w", k, err)
			}
		}(kind)
	}

	wg.Wait()
	close(errCh)

	// Return first error if any
	for err := range errCh {
		return err
	}
	return nil
}

// runWorkerLoop manages the lifecycle of a single worker kind with restart logic.
func (s *Supervisor) runWorkerLoop(ctx context.Context, kind WorkerKind) error {
	ws := &workerState{kind: kind}

	s.mu.Lock()
	s.workers[kind] = ws
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.workers, kind)
		s.mu.Unlock()
	}()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		ws.epoch++
		args := []string{
			"--daemon-worker", string(kind),
			"--epoch", fmt.Sprintf("%d", ws.epoch),
		}

		ws.cmd = exec.CommandContext(ctx, s.cfg.BinaryPath, args...)
		ws.cmd.Stdout = os.Stdout
		ws.cmd.Stderr = os.Stderr
		// Set session kind env var for child
		ws.cmd.Env = append(os.Environ(),
			"AGENT_ENGINE_SESSION_KIND=daemon-worker",
			fmt.Sprintf("AGENT_ENGINE_WORKER_EPOCH=%d", ws.epoch),
		)

		slog.Info("supervisor: starting worker",
			slog.String("kind", string(kind)),
			slog.Int("epoch", ws.epoch),
			slog.Int("restart", ws.restarts))

		if err := ws.cmd.Start(); err != nil {
			return fmt.Errorf("start worker: %w", err)
		}

		pid := ws.cmd.Process.Pid
		if s.cfg.OnWorkerStart != nil {
			s.cfg.OnWorkerStart(kind, pid, ws.epoch)
		}

		done := make(chan error, 1)
		go func() { done <- ws.cmd.Wait() }()

		select {
		case <-ctx.Done():
			if ws.cmd.Process != nil {
				_ = ws.cmd.Process.Kill()
			}
			if s.cfg.OnWorkerStop != nil {
				s.cfg.OnWorkerStop(kind, pid, ctx.Err())
			}
			return ctx.Err()

		case err := <-done:
			if s.cfg.OnWorkerStop != nil {
				s.cfg.OnWorkerStop(kind, pid, err)
			}

			if err == nil {
				slog.Info("supervisor: worker exited cleanly",
					slog.String("kind", string(kind)))
				return nil
			}

			ws.restarts++
			slog.Warn("supervisor: worker exited with error",
				slog.String("kind", string(kind)),
				slog.Any("err", err),
				slog.Int("restarts", ws.restarts))

			if ws.restarts >= s.cfg.MaxRestarts {
				return fmt.Errorf("worker %s exceeded max restarts (%d): %w",
					kind, s.cfg.MaxRestarts, err)
			}

			backoff := s.cfg.RestartBackoff * time.Duration(ws.restarts)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
}

// WorkerPID returns the current PID of a worker kind, or 0 if not running.
func (s *Supervisor) WorkerPID(kind WorkerKind) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workers[kind]
	if !ok || ws.cmd == nil || ws.cmd.Process == nil {
		return 0
	}
	return ws.cmd.Process.Pid
}

// WorkerEpoch returns the current epoch of a worker kind.
func (s *Supervisor) WorkerEpoch(kind WorkerKind) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workers[kind]
	if !ok {
		return 0
	}
	return ws.epoch
}
