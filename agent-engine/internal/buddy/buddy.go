package buddy

import (
	"math"
	"time"
)

// BuddyConfig controls the buddy (companion) system behaviour.
type BuddyConfig struct {
	// Seed for the Mulberry32 PRNG — must be deterministic per session.
	Seed uint32
	// Mood affects response tone (0.0 = neutral, 1.0 = very positive).
	Mood float64
	// Energy level (0.0 = tired, 1.0 = energetic).
	Energy float64
}

// Buddy represents the current state of the companion system.
type Buddy struct {
	cfg     BuddyConfig
	rng     *mulberry32
	created time.Time
}

// New creates a Buddy with the given config.
func New(cfg BuddyConfig) *Buddy {
	seed := cfg.Seed
	if seed == 0 {
		seed = uint32(time.Now().UnixNano() & 0xFFFFFFFF)
	}
	return &Buddy{
		cfg:     cfg,
		rng:     newMulberry32(seed),
		created: time.Now(),
	}
}

// NextFloat returns a deterministic pseudo-random float64 in [0, 1).
func (b *Buddy) NextFloat() float64 {
	return float64(b.rng.next()) / math.MaxUint32
}

// ShouldRespond returns true with probability proportional to the buddy's mood.
func (b *Buddy) ShouldRespond() bool {
	return b.NextFloat() < clamp(b.cfg.Mood, 0, 1)
}

// EnergyLevel returns a normalised energy value [0, 1].
func (b *Buddy) EnergyLevel() float64 { return clamp(b.cfg.Energy, 0, 1) }

// MoodLevel returns a normalised mood value [0, 1].
func (b *Buddy) MoodLevel() float64 { return clamp(b.cfg.Mood, 0, 1) }

// Uptime returns how long this buddy instance has been alive.
func (b *Buddy) Uptime() time.Duration { return time.Since(b.created) }

// ─── Mulberry32 PRNG ─────────────────────────────────────────────────────────
// Mulberry32 is a fast, high-quality 32-bit PRNG.
// Matches the TypeScript implementation for deterministic parity.

type mulberry32 struct{ state uint32 }

func newMulberry32(seed uint32) *mulberry32 { return &mulberry32{state: seed} }

func (m *mulberry32) next() uint32 {
	m.state += 0x6D2B79F5
	z := m.state
	z = (z ^ (z >> 15)) * (z | 1)
	z ^= z + (z^(z>>7))*(z|61)
	return z ^ (z >> 14)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
