package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionInfo holds metadata about a saved session.
type SessionInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Model       string    `json:"model"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TurnCount   int       `json:"turn_count"`
	CostUSD     float64   `json:"cost_usd"`
	WorkDir     string    `json:"work_dir"`
	MessageCount int      `json:"message_count"`
}

// SessionStore manages session persistence for resume/list/delete.
type SessionStore struct {
	baseDir string
}

// NewSessionStore creates a session store rooted at the given directory.
// Default: ~/.claude/sessions/
func NewSessionStore(baseDir string) *SessionStore {
	if baseDir == "" {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, ".claude", "sessions")
	}
	return &SessionStore{baseDir: baseDir}
}

// SaveSession writes session metadata to disk.
func (s *SessionStore) SaveSession(info SessionInfo) error {
	dir := filepath.Join(s.baseDir, info.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := filepath.Join(dir, "session.json")
	return os.WriteFile(path, data, 0o644)
}

// LoadSession reads session metadata from disk.
func (s *SessionStore) LoadSession(id string) (*SessionInfo, error) {
	path := filepath.Join(s.baseDir, id, "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}

	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &info, nil
}

// ListSessions returns all saved sessions, sorted by most recent first.
func (s *SessionStore) ListSessions() ([]SessionInfo, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := s.LoadSession(entry.Name())
		if err != nil {
			continue // skip corrupt sessions
		}
		sessions = append(sessions, *info)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// DeleteSession removes a session from disk.
func (s *SessionStore) DeleteSession(id string) error {
	dir := filepath.Join(s.baseDir, id)
	return os.RemoveAll(dir)
}

// MostRecent returns the most recently updated session, or nil.
func (s *SessionStore) MostRecent() *SessionInfo {
	sessions, err := s.ListSessions()
	if err != nil || len(sessions) == 0 {
		return nil
	}
	return &sessions[0]
}

// FormatSessionList renders a list of sessions for display.
func FormatSessionList(sessions []SessionInfo, maxShow int) string {
	if len(sessions) == 0 {
		return "No saved sessions."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Sessions (%d):\n\n", len(sessions)))

	show := sessions
	if len(show) > maxShow {
		show = show[:maxShow]
	}

	for i, s := range show {
		age := formatAge(s.UpdatedAt)
		name := s.Name
		if name == "" {
			name = s.ID[:8]
		}
		sb.WriteString(fmt.Sprintf("  %d. %s — %s (%d turns, %s)\n",
			i+1, name, s.Model, s.TurnCount, age))
		if s.WorkDir != "" {
			sb.WriteString(fmt.Sprintf("     %s\n", s.WorkDir))
		}
	}

	if len(sessions) > maxShow {
		sb.WriteString(fmt.Sprintf("\n  … and %d more\n", len(sessions)-maxShow))
	}

	return sb.String()
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
