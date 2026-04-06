package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/wall-ai/agent-engine/internal/daemon"
	"github.com/wall-ai/agent-engine/internal/daemon/ipc"
)

// ─── ps ─────────────────────────────────────────────────────────────────────

func newPsCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List active sessions",
		Long:  "List all active agent-engine sessions (interactive, daemon, workers).",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := daemon.ListSessions()
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(sessions, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(sessions) == 0 {
				fmt.Println("No active sessions.")
				return nil
			}

			// Header
			fmt.Printf("%-8s %-16s %-24s %s\n", "PID", "KIND", "SESSION", "CWD")
			fmt.Println(strings.Repeat("─", 80))
			for _, s := range sessions {
				sid := s.SessionID
				if len(sid) > 22 {
					sid = sid[:22] + "…"
				}
				cwd := s.CWD
				if len(cwd) > 30 {
					cwd = "…" + cwd[len(cwd)-29:]
				}
				fmt.Printf("%-8d %-16s %-24s %s\n", s.PID, s.Kind, sid, cwd)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

// ─── logs ───────────────────────────────────────────────────────────────────

func newLogsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs [pid]",
		Short: "View session logs",
		Long:  "View log output from a daemon worker session.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid PID: %s", args[0])
			}

			info := daemon.FindSessionByPID(pid)
			if info == nil {
				return fmt.Errorf("no session found for PID %d", pid)
			}

			if info.LogPath == "" {
				return fmt.Errorf("session PID %d has no log path configured", pid)
			}

			if follow {
				return tailLogFile(info.LogPath)
			}

			data, err := os.ReadFile(info.LogPath)
			if err != nil {
				return fmt.Errorf("read log: %w", err)
			}
			fmt.Print(string(data))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (tail -f)")
	return cmd
}

// tailLogFile reads the last portion of the file and watches for new writes.
func tailLogFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Seek to last 4KB
	info, _ := f.Stat()
	offset := info.Size() - 4096
	if offset < 0 {
		offset = 0
	}
	_, _ = f.Seek(offset, 0)

	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			fmt.Print(string(buf[:n]))
		}
		if err != nil {
			// Wait and retry (poll-based tail)
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// ─── attach ─────────────────────────────────────────────────────────────────

func newAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach [pid]",
		Short: "Attach to a daemon worker session",
		Long:  "Connect to a running daemon worker's IPC socket for interactive messaging.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid PID: %s", args[0])
			}

			info := daemon.FindSessionByPID(pid)
			if info == nil {
				return fmt.Errorf("no session found for PID %d", pid)
			}

			if info.MessagingSocketPath == "" {
				return fmt.Errorf("session PID %d has no IPC socket", pid)
			}

			client := ipc.NewClient(info.MessagingSocketPath)
			if err := client.Connect(5 * time.Second); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer client.Close()

			// Send a status request
			reply, err := client.SendAndWait(&ipc.Message{
				Type: string(ipc.MsgTypeStatus),
			}, 5*time.Second)
			if err != nil {
				return fmt.Errorf("status query: %w", err)
			}

			fmt.Printf("Attached to PID %d\n", pid)
			if reply != nil {
				data, _ := json.MarshalIndent(reply, "", "  ")
				fmt.Println(string(data))
			}

			fmt.Println("\nUse Ctrl+C to detach.")

			// Read loop to show streamed messages
			client.OnMessage(string(ipc.MsgTypeLog), func(msg *ipc.Message) *ipc.Message {
				if msg.Payload != nil {
					fmt.Printf("[log] %s\n", string(msg.Payload))
				}
				return nil
			})
			return client.ReadLoop()
		},
	}
}

// ─── kill ───────────────────────────────────────────────────────────────────

func newKillCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "kill [pid]",
		Short: "Terminate a session",
		Long:  "Send a termination signal to a running session by PID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid PID: %s", args[0])
			}

			info := daemon.FindSessionByPID(pid)
			if info == nil {
				return fmt.Errorf("no session found for PID %d", pid)
			}

			// Try IPC graceful shutdown first (if socket available and not --force)
			if !force && info.MessagingSocketPath != "" {
				client := ipc.NewClient(info.MessagingSocketPath)
				if err := client.Connect(2 * time.Second); err == nil {
					_ = client.Send(&ipc.Message{
						Type: string(ipc.MsgTypeShutdown),
					})
					client.Close()
					fmt.Printf("Sent shutdown request to PID %d\n", pid)
					return nil
				}
			}

			// Fallback to signal
			proc, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("find process: %w", err)
			}

			sig := syscall.SIGTERM
			if force {
				sig = syscall.SIGKILL
			}

			if err := proc.Signal(sig); err != nil {
				return fmt.Errorf("signal %d: %w", pid, err)
			}

			sigName := "SIGTERM"
			if force {
				sigName = "SIGKILL"
			}
			fmt.Printf("Sent %s to PID %d\n", sigName, pid)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force kill (SIGKILL)")
	return cmd
}
