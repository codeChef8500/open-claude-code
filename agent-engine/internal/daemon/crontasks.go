package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CronTask represents a scheduled prompt task.
// Aligned with claude-code-main utils/cronTasks.ts CronTask type.
type CronTask struct {
	ID          string `json:"id"`
	Cron        string `json:"cron"`
	Prompt      string `json:"prompt"`
	CreatedAt   int64  `json:"createdAt"`
	LastFiredAt int64  `json:"lastFiredAt,omitempty"`
	Recurring   bool   `json:"recurring,omitempty"`
	Permanent   bool   `json:"permanent,omitempty"`
	Durable     bool   `json:"durable,omitempty"`
	AgentID     string `json:"agentId,omitempty"`
}

// CronTaskStore provides dual-track storage for cron tasks:
//   - session-only tasks: in-memory only, lost on process exit
//   - durable tasks: persisted to .claude/scheduled_tasks.json
//
// Aligned with claude-code-main utils/cronTasks.ts.
type CronTaskStore struct {
	mu sync.RWMutex

	// session-only tasks (in-memory)
	sessionTasks map[string]*CronTask

	// filePath is the JSON file for durable tasks.
	filePath string
}

// NewCronTaskStore creates a CronTaskStore.
// filePath is the path to .claude/scheduled_tasks.json (or equivalent).
func NewCronTaskStore(filePath string) *CronTaskStore {
	return &CronTaskStore{
		sessionTasks: make(map[string]*CronTask),
		filePath:     filePath,
	}
}

// Add creates and stores a new CronTask, returning its ID.
func (s *CronTaskStore) Add(cron, prompt string, recurring, durable bool, agentID string) (string, error) {
	id := generateTaskID()
	task := &CronTask{
		ID:        id,
		Cron:      cron,
		Prompt:    prompt,
		CreatedAt: time.Now().UnixMilli(),
		Recurring: recurring,
		Durable:   durable,
		AgentID:   agentID,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if durable {
		tasks, err := s.readDurableFile()
		if err != nil {
			tasks = []*CronTask{}
		}
		tasks = append(tasks, task)
		if err := s.writeDurableFile(tasks); err != nil {
			return "", fmt.Errorf("persist durable task: %w", err)
		}
	} else {
		s.sessionTasks[id] = task
	}

	return id, nil
}

// Remove deletes one or more tasks by ID from both stores.
func (s *CronTaskStore) Remove(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	// Remove from session store
	for _, id := range ids {
		delete(s.sessionTasks, id)
	}

	// Remove from durable file
	tasks, err := s.readDurableFile()
	if err != nil {
		return nil // no file = nothing to remove
	}
	filtered := make([]*CronTask, 0, len(tasks))
	for _, t := range tasks {
		if _, found := idSet[t.ID]; !found {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) != len(tasks) {
		return s.writeDurableFile(filtered)
	}
	return nil
}

// ListAll returns all tasks from both session and durable stores.
func (s *CronTaskStore) ListAll() []*CronTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*CronTask

	// Session-only tasks
	for _, t := range s.sessionTasks {
		out = append(out, t)
	}

	// Durable tasks
	durable, err := s.readDurableFile()
	if err == nil {
		out = append(out, durable...)
	}

	return out
}

// ListDurable returns only durable tasks from the JSON file.
func (s *CronTaskStore) ListDurable() []*CronTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks, err := s.readDurableFile()
	if err != nil {
		return nil
	}
	return tasks
}

// MarkFired updates the lastFiredAt timestamp for a task.
func (s *CronTaskStore) MarkFired(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nowMs := time.Now().UnixMilli()

	// Check session store
	if t, ok := s.sessionTasks[taskID]; ok {
		t.LastFiredAt = nowMs
		return
	}

	// Check durable store
	tasks, err := s.readDurableFile()
	if err != nil {
		return
	}
	for _, t := range tasks {
		if t.ID == taskID {
			t.LastFiredAt = nowMs
			_ = s.writeDurableFile(tasks)
			return
		}
	}
}

// FindMissed returns durable tasks whose next fire time is in the past
// (i.e., they were missed while the process was not running).
func (s *CronTaskStore) FindMissed(nowMs int64) []*CronTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks, err := s.readDurableFile()
	if err != nil {
		return nil
	}

	var missed []*CronTask
	for _, t := range tasks {
		if !t.Recurring && t.LastFiredAt > 0 {
			continue // one-shot already fired
		}
		baseMs := t.LastFiredAt
		if baseMs == 0 {
			baseMs = t.CreatedAt
		}
		nextMs := NextRunMs(t.Cron, baseMs)
		if nextMs > 0 && nextMs < nowMs {
			missed = append(missed, t)
		}
	}
	return missed
}

// Find looks up a single task by ID across both stores.
func (s *CronTaskStore) Find(id string) *CronTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if t, ok := s.sessionTasks[id]; ok {
		return t
	}

	tasks, err := s.readDurableFile()
	if err != nil {
		return nil
	}
	for _, t := range tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// RemoveExpired removes durable recurring tasks older than maxAgeMs.
// Returns the list of removed task IDs.
func (s *CronTaskStore) RemoveExpired(maxAgeMs int64) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.readDurableFile()
	if err != nil {
		return nil
	}

	nowMs := time.Now().UnixMilli()
	var kept []*CronTask
	var removed []string

	for _, t := range tasks {
		if t.Recurring && !t.Permanent && (nowMs-t.CreatedAt) > maxAgeMs {
			removed = append(removed, t.ID)
			slog.Info("crontasks: expiring aged task", slog.String("id", t.ID))
		} else {
			kept = append(kept, t)
		}
	}

	if len(removed) > 0 {
		_ = s.writeDurableFile(kept)
	}
	return removed
}

// Count returns the total number of tasks across both stores.
func (s *CronTaskStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := len(s.sessionTasks)
	tasks, err := s.readDurableFile()
	if err == nil {
		n += len(tasks)
	}
	return n
}

// FilePath returns the durable JSON file path.
func (s *CronTaskStore) FilePath() string {
	return s.filePath
}

// ─── internal persistence ───────────────────────────────────────────────────

func (s *CronTaskStore) readDurableFile() ([]*CronTask, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, err
	}
	var tasks []*CronTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *CronTaskStore) writeDurableFile(tasks []*CronTask) error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write via temp file + rename
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func generateTaskID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GetDefaultCronFilePath returns the default path for scheduled_tasks.json.
func GetDefaultCronFilePath(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "scheduled_tasks.json")
}
