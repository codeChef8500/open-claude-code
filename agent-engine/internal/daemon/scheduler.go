package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler wraps robfig/cron and persists tasks to disk for durability.
type Scheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	tasks   map[string]*scheduledEntry
	dataDir string
	onRun   func(task *ScheduledTask)
}

type scheduledEntry struct {
	task  *ScheduledTask
	entID cron.EntryID
}

// NewScheduler creates a Scheduler that persists tasks under dataDir.
// onRun is called each time a task fires.
func NewScheduler(dataDir string, onRun func(*ScheduledTask)) *Scheduler {
	return &Scheduler{
		cron:    cron.New(cron.WithSeconds()),
		tasks:   make(map[string]*scheduledEntry),
		dataDir: dataDir,
		onRun:   onRun,
	}
}

// Start launches the cron scheduler and re-loads durable tasks from disk.
func (s *Scheduler) Start(ctx context.Context) error {
	s.cron.Start()
	s.loadDurable()
	<-ctx.Done()
	s.cron.Stop()
	return nil
}

// Add registers a new ScheduledTask.
func (s *Scheduler) Add(task *ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, err := s.cron.AddFunc(task.CronExpr, func() {
		task.LastRunAt = time.Now()
		task.RunCount++
		if s.onRun != nil {
			s.onRun(task)
		}
		if task.Durable {
			_ = s.persist(task)
		}
	})
	if err != nil {
		return fmt.Errorf("scheduler add: %w", err)
	}

	s.tasks[task.ID] = &scheduledEntry{task: task, entID: id}

	if task.Durable {
		return s.persist(task)
	}
	return nil
}

// Remove cancels and removes a scheduled task.
func (s *Scheduler) Remove(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	s.cron.Remove(e.entID)
	delete(s.tasks, taskID)
	_ = s.deleteFile(taskID)
	return nil
}

// List returns all registered tasks.
func (s *Scheduler) List() []*ScheduledTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*ScheduledTask, 0, len(s.tasks))
	for _, e := range s.tasks {
		out = append(out, e.task)
	}
	return out
}

// ─── persistence ──────────────────────────────────────────────────────────────

func (s *Scheduler) persist(task *ScheduledTask) error {
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.taskPath(task.ID), b, 0o644)
}

func (s *Scheduler) deleteFile(taskID string) error {
	return os.Remove(s.taskPath(taskID))
}

func (s *Scheduler) taskPath(taskID string) string {
	return filepath.Join(s.dataDir, "task-"+taskID+".json")
}

func (s *Scheduler) loadDurable() {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dataDir, e.Name()))
		if err != nil {
			continue
		}
		var task ScheduledTask
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}
		if !task.Durable {
			continue
		}
		if !task.ExpiresAt.IsZero() && time.Now().After(task.ExpiresAt) {
			slog.Info("scheduler: expiring task", slog.String("id", task.ID))
			_ = s.deleteFile(task.ID)
			continue
		}
		if err := s.Add(&task); err != nil {
			slog.Warn("scheduler: failed to restore task",
				slog.String("id", task.ID), slog.Any("err", err))
		}
	}
}
