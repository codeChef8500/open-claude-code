package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/wall-ai/agent-engine/internal/daemon"
	"github.com/wall-ai/agent-engine/internal/state"
	"github.com/wall-ai/agent-engine/internal/util"
)

// daemonOpts holds daemon-specific flags.
type daemonOpts struct {
	WorkerKind string
	Epoch      int
	SessionID  string
}

var dOpts daemonOpts

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Start the KAIROS assistant daemon (supervisor mode)",
		Long: `Start agent-engine as a KAIROS assistant daemon.
The supervisor spawns and manages worker processes that run scheduled tasks,
handle proactive checks, and maintain session state.`,
		RunE:          runDaemonSupervisor,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	f := cmd.Flags()
	f.StringVar(&dOpts.SessionID, "session-id", "", "Session ID for this daemon instance")

	// Hidden flags used when supervisor spawns workers
	cmd.AddCommand(newDaemonWorkerCmd())
	cmd.AddCommand(newDaemonStatusCmd())
	cmd.AddCommand(newDaemonStopCmd())

	return cmd
}

// ─── Supervisor ─────────────────────────────────────────────────────────────

func runDaemonSupervisor(cmd *cobra.Command, args []string) error {
	if err := util.InitConfig(); err != nil {
		return fmt.Errorf("init config: %w", err)
	}
	util.InitLogger(true)

	wd, _ := os.Getwd()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Register this supervisor session
	cleanup, regErr := daemon.RegisterSession(daemon.SessionInfo{
		PID:       os.Getpid(),
		SessionID: dOpts.SessionID,
		CWD:       wd,
		Kind:      string(state.SessionKindDaemon),
	}, "")
	if regErr != nil {
		slog.Warn("daemon: failed to register PID", slog.Any("err", regErr))
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Resolve binary path for spawning workers
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	slog.Info("daemon: starting supervisor",
		slog.String("binary", binaryPath),
		slog.String("cwd", wd))

	sup := daemon.NewSupervisor(daemon.SupervisorConfig{
		BinaryPath:  binaryPath,
		WorkerKinds: []daemon.WorkerKind{daemon.WorkerKindAssistant},
		OnWorkerStart: func(kind daemon.WorkerKind, pid int, epoch int) {
			slog.Info("daemon: worker started",
				slog.String("kind", string(kind)),
				slog.Int("pid", pid),
				slog.Int("epoch", epoch))
		},
		OnWorkerStop: func(kind daemon.WorkerKind, pid int, err error) {
			if err != nil {
				slog.Warn("daemon: worker stopped",
					slog.String("kind", string(kind)),
					slog.Int("pid", pid),
					slog.Any("err", err))
			} else {
				slog.Info("daemon: worker stopped cleanly",
					slog.String("kind", string(kind)),
					slog.Int("pid", pid))
			}
		},
	})

	return sup.Run(ctx)
}

// ─── Worker (spawned by supervisor) ─────────────────────────────────────────

func newDaemonWorkerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "worker",
		Short:  "Run as a daemon worker (called by supervisor)",
		Hidden: true,
		RunE:   runDaemonWorker,
	}
	f := cmd.Flags()
	f.StringVar(&dOpts.WorkerKind, "kind", "assistant", "Worker kind")
	f.IntVar(&dOpts.Epoch, "epoch", 1, "Worker epoch")
	return cmd
}

func runDaemonWorker(cmd *cobra.Command, args []string) error {
	if err := util.InitConfig(); err != nil {
		return fmt.Errorf("init config: %w", err)
	}
	util.InitLogger(true)

	slog.Info("daemon-worker: starting",
		slog.String("kind", dOpts.WorkerKind),
		slog.Int("epoch", dOpts.Epoch))

	// Dispatch to the worker registry
	return daemon.RunDaemonWorker(dOpts.WorkerKind)
}

// ─── Status ─────────────────────────────────────────────────────────────────

func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon and session status",
		RunE:  runDaemonStatus,
	}
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	sessions, err := daemon.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No active daemon sessions.")
		return nil
	}

	fmt.Printf("Active sessions (%d):\n", len(sessions))
	for _, s := range sessions {
		fmt.Printf("  PID %-6d  %-15s  session=%s  cwd=%s\n",
			s.PID, s.Kind, s.SessionID, s.CWD)
	}
	return nil
}

// ─── Stop ───────────────────────────────────────────────────────────────────

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running daemon",
		RunE:  runDaemonStop,
	}
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	sessions, err := daemon.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	stopped := 0
	for _, s := range sessions {
		if s.Kind == string(state.SessionKindDaemon) || s.Kind == string(state.SessionKindDaemonWorker) {
			proc, err := os.FindProcess(s.PID)
			if err != nil {
				continue
			}
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				slog.Warn("daemon stop: signal failed",
					slog.Int("pid", s.PID),
					slog.Any("err", err))
				continue
			}
			stopped++
			fmt.Printf("Sent SIGTERM to PID %d (%s)\n", s.PID, s.Kind)
		}
	}

	if stopped == 0 {
		fmt.Println("No daemon processes found.")
	}
	return nil
}
