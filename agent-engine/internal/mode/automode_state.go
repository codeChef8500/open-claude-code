package mode

import (
	"sync/atomic"
)

// AutoModeState tracks cumulative classifier decisions for the lifetime of a
// session. All fields use atomic operations so the struct is safe for
// concurrent access without a mutex.
type AutoModeState struct {
	allows    atomic.Int64
	softDenys atomic.Int64
	denys     atomic.Int64
}

// Record increments the counter for the given verdict.
func (s *AutoModeState) Record(verdict ClassifierVerdict) {
	switch verdict {
	case VerdictAllow:
		s.allows.Add(1)
	case VerdictSoftDeny:
		s.softDenys.Add(1)
	case VerdictDeny:
		s.denys.Add(1)
	}
}

// Stats returns a snapshot of the current counters.
func (s *AutoModeState) Stats() AutoModeStats {
	return AutoModeStats{
		Allows:    int(s.allows.Load()),
		SoftDenys: int(s.softDenys.Load()),
		Denys:     int(s.denys.Load()),
	}
}

// Total returns the total number of classifier decisions recorded.
func (s *AutoModeState) Total() int {
	return int(s.allows.Load() + s.softDenys.Load() + s.denys.Load())
}

// AutoModeStats is a value snapshot of AutoModeState counters.
type AutoModeStats struct {
	Allows    int `json:"allows"`
	SoftDenys int `json:"soft_denys"`
	Denys     int `json:"denys"`
}
