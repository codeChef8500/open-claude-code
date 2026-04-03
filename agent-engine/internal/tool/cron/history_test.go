package cron

import (
	"testing"
	"time"
)

func TestJobHistory_Record(t *testing.T) {
	h := NewJobHistory(10)
	h.Record(JobRun{JobID: "j1", StartedAt: time.Now(), Duration: time.Second, Success: true})
	if h.Count() != 1 {
		t.Errorf("expected 1, got %d", h.Count())
	}
}

func TestJobHistory_Cap(t *testing.T) {
	h := NewJobHistory(5)
	for i := 0; i < 10; i++ {
		h.Record(JobRun{JobID: "j", StartedAt: time.Now(), Duration: time.Millisecond, Success: true})
	}
	if h.Count() != 5 {
		t.Errorf("expected capped at 5, got %d", h.Count())
	}
}

func TestJobHistory_ForJob(t *testing.T) {
	h := NewJobHistory(100)
	h.Record(JobRun{JobID: "a", Success: true})
	h.Record(JobRun{JobID: "b", Success: true})
	h.Record(JobRun{JobID: "a", Success: false})

	runs := h.ForJob("a")
	if len(runs) != 2 {
		t.Errorf("expected 2 runs for job 'a', got %d", len(runs))
	}
	runs = h.ForJob("c")
	if len(runs) != 0 {
		t.Errorf("expected 0 runs for job 'c', got %d", len(runs))
	}
}

func TestJobHistory_Recent(t *testing.T) {
	h := NewJobHistory(100)
	for i := 0; i < 10; i++ {
		h.Record(JobRun{JobID: "j", Output: "run"})
	}

	recent := h.Recent(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent, got %d", len(recent))
	}

	all := h.Recent(0)
	if len(all) != 10 {
		t.Errorf("expected 10 for n=0, got %d", len(all))
	}

	over := h.Recent(100)
	if len(over) != 10 {
		t.Errorf("expected 10 for n>count, got %d", len(over))
	}
}

func TestJobHistory_Stats(t *testing.T) {
	h := NewJobHistory(100)
	h.Record(JobRun{JobID: "j", Duration: 2 * time.Second, Success: true})
	h.Record(JobRun{JobID: "j", Duration: 4 * time.Second, Success: true})
	h.Record(JobRun{JobID: "j", Duration: 3 * time.Second, Success: false})

	stats := h.Stats()
	if stats.TotalRuns != 3 {
		t.Errorf("expected 3 total, got %d", stats.TotalRuns)
	}
	if stats.Successes != 2 {
		t.Errorf("expected 2 successes, got %d", stats.Successes)
	}
	if stats.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", stats.Failures)
	}
	if stats.TotalRuntime != 9*time.Second {
		t.Errorf("expected 9s total runtime, got %v", stats.TotalRuntime)
	}
	if stats.AvgRuntime != 3*time.Second {
		t.Errorf("expected 3s avg runtime, got %v", stats.AvgRuntime)
	}
}

func TestJobHistory_StatsEmpty(t *testing.T) {
	h := NewJobHistory(100)
	stats := h.Stats()
	if stats.TotalRuns != 0 || stats.AvgRuntime != 0 {
		t.Error("expected zero stats for empty history")
	}
}

func TestJobHistory_Clear(t *testing.T) {
	h := NewJobHistory(100)
	h.Record(JobRun{JobID: "j"})
	h.Record(JobRun{JobID: "j"})
	h.Clear()
	if h.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", h.Count())
	}
}

func TestNewJobHistory_DefaultMax(t *testing.T) {
	h := NewJobHistory(0)
	if h.maxRuns != 500 {
		t.Errorf("expected default 500, got %d", h.maxRuns)
	}
}
