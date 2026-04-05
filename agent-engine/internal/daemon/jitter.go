package daemon

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// JitterConfig controls how task fire times are jittered to avoid thundering-
// herd problems. Aligned with claude-code-main utils/cronJitterConfig.ts.
type JitterConfig struct {
	// RecurringFrac is the fraction of the period to jitter (e.g. 0.10 = 10%).
	RecurringFrac float64
	// RecurringCapMs caps the absolute recurring jitter in milliseconds.
	RecurringCapMs int64
	// OneShotMaxMs is the maximum jitter for one-shot tasks on :00/:30.
	OneShotMaxMs int64
	// OneShotFloorMs is the minimum jitter for one-shot tasks.
	OneShotFloorMs int64
	// OneShotMinuteMod controls which minutes trigger one-shot jitter (0 and 30).
	OneShotMinuteMod int
	// RecurringMaxAgeMs is the auto-expiry age for recurring tasks.
	RecurringMaxAgeMs int64
}

// DefaultJitterConfig matches claude-code-main's DEFAULT_CRON_JITTER_CONFIG.
var DefaultJitterConfig = JitterConfig{
	RecurringFrac:     0.10,
	RecurringCapMs:    15 * 60 * 1000, // 15 minutes
	OneShotMaxMs:      90 * 1000,      // 90 seconds
	OneShotFloorMs:    5 * 1000,       // 5 seconds
	OneShotMinuteMod:  30,             // trigger on :00 and :30
	RecurringMaxAgeMs: 14 * 24 * 60 * 60 * 1000, // 14 days
}

// JitteredNextCronRunMs computes the next fire time for a recurring task with
// deterministic jitter. The jitter is derived from a hash of the cron expression
// so that all instances with the same expression jitter by the same amount.
//
// Ported from claude-code-main utils/cronTasks.ts jitteredNextCronRunMs.
func JitteredNextCronRunMs(cronExpr string, nowMs int64, cfg JitterConfig) int64 {
	baseMs := NextRunMs(cronExpr, nowMs)
	if baseMs < 0 {
		return -1
	}

	// Compute period: distance from base to next-next-run
	nextNextMs := NextRunMs(cronExpr, baseMs)
	if nextNextMs < 0 {
		return baseMs
	}
	periodMs := nextNextMs - baseMs
	if periodMs <= 0 {
		return baseMs
	}

	// Jitter = min(frac * period, cap)
	maxJitter := int64(math.Min(
		cfg.RecurringFrac*float64(periodMs),
		float64(cfg.RecurringCapMs),
	))
	if maxJitter <= 0 {
		return baseMs
	}

	jitter := deterministicJitter(cronExpr, maxJitter)
	return baseMs + jitter
}

// OneShotJitteredNextCronRunMs computes the next fire time for a one-shot task.
// If the minute lands on :00 or :30, apply a small random jitter to spread
// load. Otherwise return the base time unchanged.
//
// Ported from claude-code-main utils/cronTasks.ts oneShotJitteredNextCronRunMs.
func OneShotJitteredNextCronRunMs(cronExpr string, nowMs int64, cfg JitterConfig) int64 {
	baseMs := NextRunMs(cronExpr, nowMs)
	if baseMs < 0 {
		return -1
	}

	// Only jitter if the fire minute is on a :00/:30 boundary
	fields, err := ParseCronExpression(cronExpr)
	if err != nil {
		return baseMs
	}

	needsJitter := false
	for _, m := range fields.Minute {
		if cfg.OneShotMinuteMod > 0 && m%cfg.OneShotMinuteMod == 0 {
			needsJitter = true
			break
		}
	}
	if !needsJitter {
		return baseMs
	}

	jitter := deterministicJitter(cronExpr, cfg.OneShotMaxMs)
	if jitter < cfg.OneShotFloorMs {
		jitter = cfg.OneShotFloorMs
	}

	// One-shot jitter goes backward (fire slightly early) to avoid clustering
	return baseMs - jitter
}

// deterministicJitter returns a value in [0, maxMs) derived from a SHA-256
// hash of the cron expression. Deterministic: same expression always yields
// the same jitter, spreading different expressions across the interval.
func deterministicJitter(cronExpr string, maxMs int64) int64 {
	if maxMs <= 0 {
		return 0
	}
	h := sha256.Sum256([]byte(cronExpr))
	n := binary.BigEndian.Uint64(h[:8])
	return int64(n % uint64(maxMs))
}
