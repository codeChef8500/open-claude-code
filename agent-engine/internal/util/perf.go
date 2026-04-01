package util

import (
	"encoding/json"
	"log/slog"
	"time"
)

const defaultSlowThreshold = 100 * time.Millisecond

var slowThreshold = defaultSlowThreshold

// SetSlowThreshold overrides the threshold above which operations are reported
// as slow. Default is 100ms.
func SetSlowThreshold(d time.Duration) {
	slowThreshold = d
}

// SlowJSONMarshal serialises value to JSON and logs a warning if it takes
// longer than the slow threshold.
func SlowJSONMarshal(value interface{}) ([]byte, error) {
	start := time.Now()
	b, err := json.Marshal(value)
	elapsed := time.Since(start)
	if elapsed > slowThreshold {
		slog.Warn("slow json.Marshal",
			slog.Duration("elapsed", elapsed),
			slog.String("type", typeNameOf(value)),
			slog.Int("size", len(b)),
		)
	}
	return b, err
}

// TrackOperation calls fn and logs a warning if it exceeds the slow threshold.
// name is used for identification in the log.
func TrackOperation(name string, fn func()) {
	start := time.Now()
	fn()
	elapsed := time.Since(start)
	if elapsed > slowThreshold {
		slog.Warn("slow operation",
			slog.String("operation", name),
			slog.Duration("elapsed", elapsed),
		)
	}
}

// TrackOperationErr is like TrackOperation but fn may return an error.
func TrackOperationErr(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	elapsed := time.Since(start)
	if elapsed > slowThreshold {
		slog.Warn("slow operation",
			slog.String("operation", name),
			slog.Duration("elapsed", elapsed),
		)
	}
	return err
}

func typeNameOf(v interface{}) string {
	if v == nil {
		return "nil"
	}
	switch v.(type) {
	case string:
		return "string"
	case []byte:
		return "[]byte"
	case map[string]interface{}:
		return "map"
	default:
		return "unknown"
	}
}
