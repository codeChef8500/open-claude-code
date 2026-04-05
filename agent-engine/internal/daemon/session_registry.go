package daemon

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/util"
)

// SessionInfo is the JSON structure stored in each PID file under
// ~/.agent-engine/sessions/<pid>.json.
// Aligned with claude-code-main utils/concurrentSessions.ts.
type SessionInfo struct {
	PID                 int    `json:"pid"`
	SessionID           string `json:"session_id"`
	CWD                 string `json:"cwd"`
	StartedAt           int64  `json:"started_at"`
	Kind                string `json:"kind"` // SessionKind constant
	Entrypoint          string `json:"entrypoint,omitempty"`
	MessagingSocketPath string `json:"messaging_socket_path,omitempty"`
	Name                string `json:"name,omitempty"`
	LogPath             string `json:"log_path,omitempty"`
	Agent               string `json:"agent,omitempty"`
}

// GetSessionsDir returns the directory for session PID files.
func GetSessionsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agent-engine", "sessions")
}

// RegisterSession writes a session PID file and returns a cleanup function
// that removes it. Teammate sub-processes (agentID != "") do NOT register
// PID files to avoid clutter, matching claude-code behavior.
func RegisterSession(info SessionInfo, agentID string) (cleanup func(), err error) {
	// Teammates don't register PID files
	if agentID != "" {
		return func() {}, nil
	}

	dir := GetSessionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("session registry: mkdir: %w", err)
	}

	if info.PID == 0 {
		info.PID = os.Getpid()
	}
	if info.StartedAt == 0 {
		info.StartedAt = time.Now().UnixMilli()
	}

	pidFile := filepath.Join(dir, fmt.Sprintf("%d.json", info.PID))

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("session registry: marshal: %w", err)
	}

	if err := os.WriteFile(pidFile, data, 0o644); err != nil {
		return nil, fmt.Errorf("session registry: write: %w", err)
	}

	slog.Debug("session registered",
		slog.Int("pid", info.PID),
		slog.String("kind", info.Kind),
		slog.String("session_id", info.SessionID))

	cleanupFn := func() {
		if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
			slog.Warn("session registry: cleanup failed", slog.Any("err", err))
		}
	}

	// Register with util cleanup system
	util.RegisterCleanup(cleanupFn)

	return cleanupFn, nil
}

// UpdateSessionInfo updates specific fields in an existing PID file.
func UpdateSessionInfo(pid int, update func(info *SessionInfo)) error {
	dir := GetSessionsDir()
	pidFile := filepath.Join(dir, fmt.Sprintf("%d.json", pid))

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return err
	}

	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return err
	}

	update(&info)

	newData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pidFile, newData, 0o644)
}

// ListSessions reads all session PID files, cleans up stale entries
// (processes that are no longer running), and returns the live sessions.
func ListSessions() ([]SessionInfo, error) {
	dir := GetSessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var live []SessionInfo

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var info SessionInfo
		if err := json.Unmarshal(data, &info); err != nil {
			// Corrupt file — remove
			_ = os.Remove(filepath.Join(dir, entry.Name()))
			continue
		}

		if info.PID <= 0 {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
			continue
		}

		// Check if process is still alive
		if !util.IsProcessAlive(info.PID) {
			slog.Debug("session registry: cleaning stale PID",
				slog.Int("pid", info.PID))
			_ = os.Remove(filepath.Join(dir, entry.Name()))
			continue
		}

		live = append(live, info)
	}

	return live, nil
}

// CountConcurrentSessions returns the number of currently active sessions,
// cleaning up stale entries in the process.
func CountConcurrentSessions() (int, error) {
	sessions, err := ListSessions()
	if err != nil {
		return 0, err
	}
	return len(sessions), nil
}

// FindSession finds a session by its session ID.
func FindSession(sessionID string) *SessionInfo {
	sessions, err := ListSessions()
	if err != nil {
		return nil
	}
	for i := range sessions {
		if sessions[i].SessionID == sessionID {
			return &sessions[i]
		}
	}
	return nil
}

// FindSessionByPID finds a session by its PID.
func FindSessionByPID(pid int) *SessionInfo {
	dir := GetSessionsDir()
	pidFile := filepath.Join(dir, fmt.Sprintf("%d.json", pid))

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil
	}

	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil
	}
	return &info
}

// ListSessionsByKind returns sessions filtered by session kind.
func ListSessionsByKind(kind string) ([]SessionInfo, error) {
	sessions, err := ListSessions()
	if err != nil {
		return nil, err
	}
	var filtered []SessionInfo
	for _, s := range sessions {
		if s.Kind == kind {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}
