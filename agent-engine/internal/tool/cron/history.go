package cron

import (
	"sync"
	"time"
)

// JobRun records the result of a single cron job execution.
type JobRun struct {
	JobID     string        `json:"job_id"`
	StartedAt time.Time    `json:"started_at"`
	Duration  time.Duration `json:"duration"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Success   bool          `json:"success"`
}

// JobStats holds aggregate execution statistics.
type JobStats struct {
	TotalRuns    int           `json:"total_runs"`
	Successes    int           `json:"successes"`
	Failures     int           `json:"failures"`
	TotalRuntime time.Duration `json:"total_runtime"`
	AvgRuntime   time.Duration `json:"avg_runtime"`
}

// JobHistory tracks execution history for all cron jobs.
type JobHistory struct {
	mu      sync.RWMutex
	runs    []JobRun
	maxRuns int
}

// NewJobHistory creates a job history tracker with the given max entries.
func NewJobHistory(maxRuns int) *JobHistory {
	if maxRuns <= 0 {
		maxRuns = 500
	}
	return &JobHistory{
		runs:    make([]JobRun, 0, maxRuns),
		maxRuns: maxRuns,
	}
}

// Record adds a job run to the history.
func (h *JobHistory) Record(run JobRun) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.runs = append(h.runs, run)
	if len(h.runs) > h.maxRuns {
		h.runs = h.runs[len(h.runs)-h.maxRuns:]
	}
}

// ForJob returns all runs for a specific job ID.
func (h *JobHistory) ForJob(jobID string) []JobRun {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []JobRun
	for _, r := range h.runs {
		if r.JobID == jobID {
			out = append(out, r)
		}
	}
	return out
}

// Recent returns the N most recent runs across all jobs.
func (h *JobHistory) Recent(n int) []JobRun {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if n <= 0 || n > len(h.runs) {
		n = len(h.runs)
	}
	start := len(h.runs) - n
	out := make([]JobRun, n)
	copy(out, h.runs[start:])
	return out
}

// Stats returns aggregate statistics.
func (h *JobHistory) Stats() JobStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var stats JobStats
	stats.TotalRuns = len(h.runs)
	for _, r := range h.runs {
		if r.Success {
			stats.Successes++
		} else {
			stats.Failures++
		}
		stats.TotalRuntime += r.Duration
	}
	if stats.TotalRuns > 0 {
		stats.AvgRuntime = stats.TotalRuntime / time.Duration(stats.TotalRuns)
	}
	return stats
}

// Clear removes all history entries.
func (h *JobHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.runs = h.runs[:0]
}

// Count returns the total number of recorded runs.
func (h *JobHistory) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.runs)
}
