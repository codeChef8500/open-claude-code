package buddy

import (
	"math"
	"sync"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const Salt = "friend-2026-401"

// ─── Mulberry32 PRNG ─────────────────────────────────────────────────────────
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

// ─── FNV-1a Hash ─────────────────────────────────────────────────────────────
// Matches the non-Bun branch of companion.ts hashString().

func hashString(s string) uint32 {
	h := uint32(2166136261) // FNV offset basis
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h = imul(h, 16777619) // FNV prime
	}
	return h
}

// imul replicates Math.imul — 32-bit multiply with wrap.
func imul(a, b uint32) uint32 {
	return uint32(int32(a) * int32(b))
}

// ─── PRNG helpers ────────────────────────────────────────────────────────────

// mulberry32Float wraps mulberry32 into a closure returning float64 in [0,1).
// Matches the TypeScript: ((t ^ (t >>> 14)) >>> 0) / 4294967296
func mulberry32Float(seed uint32) func() float64 {
	m := newMulberry32(seed)
	return func() float64 {
		return float64(m.next()) / 4294967296.0
	}
}

// pick selects a random element from a slice using rng.
func pickSpecies(rng func() float64, arr []Species) Species {
	return arr[int(math.Floor(rng()*float64(len(arr))))]
}

func pickEye(rng func() float64, arr []Eye) Eye {
	return arr[int(math.Floor(rng()*float64(len(arr))))]
}

func pickHat(rng func() float64, arr []Hat) Hat {
	return arr[int(math.Floor(rng()*float64(len(arr))))]
}

func pickStatName(rng func() float64, arr []StatName) StatName {
	return arr[int(math.Floor(rng()*float64(len(arr))))]
}

// ─── Roll functions ──────────────────────────────────────────────────────────

// rollRarity selects a rarity using weighted random (total weight = 100).
// Matches companion.ts rollRarity() exactly.
func rollRarity(rng func() float64) Rarity {
	total := 0
	for _, r := range AllRarities {
		total += RarityWeights[r]
	}
	roll := rng() * float64(total)
	for _, r := range AllRarities {
		roll -= float64(RarityWeights[r])
		if roll < 0 {
			return r
		}
	}
	return RarityCommon
}

// rollStats generates the 5-dimension stat block.
// One peak stat, one dump stat, rest scattered. Rarity bumps the floor.
// Matches companion.ts rollStats() exactly.
func rollStats(rng func() float64, rarity Rarity) map[StatName]int {
	floor := RarityFloor[rarity]
	peak := pickStatName(rng, AllStatNames)
	dump := pickStatName(rng, AllStatNames)
	for dump == peak {
		dump = pickStatName(rng, AllStatNames)
	}

	stats := make(map[StatName]int, len(AllStatNames))
	for _, name := range AllStatNames {
		switch name {
		case peak:
			v := floor + 50 + int(math.Floor(rng()*30))
			if v > 100 {
				v = 100
			}
			stats[name] = v
		case dump:
			v := floor - 10 + int(math.Floor(rng()*15))
			if v < 1 {
				v = 1
			}
			stats[name] = v
		default:
			stats[name] = floor + int(math.Floor(rng()*40))
		}
	}
	return stats
}

// ─── Roll result ─────────────────────────────────────────────────────────────

// Roll holds the result of rolling a companion from a userId.
type Roll struct {
	Bones           CompanionBones
	InspirationSeed int
}

// rollFrom executes the full roll sequence from an rng.
// Order MUST match companion.ts rollFrom() exactly:
//
//	rarity → species → eye → hat → shiny → stats → inspirationSeed
func rollFrom(rng func() float64) Roll {
	rarity := rollRarity(rng)
	// Order MUST consume rng in exact same sequence as companion.ts:
	// ② species → ③ eye → ④ hat → ⑤ shiny → ⑥ stats → ⑦ inspirationSeed
	species := pickSpecies(rng, AllSpecies)
	eye := pickEye(rng, AllEyes)
	hat := HatNone
	if rarity != RarityCommon {
		hat = pickHat(rng, AllHats)
	}
	bones := CompanionBones{
		Rarity:  rarity,
		Species: species,
		Eye:     eye,
		Hat:     hat,
		Shiny:   rng() < 0.01,
		Stats:   rollStats(rng, rarity),
	}
	return Roll{
		Bones:           bones,
		InspirationSeed: int(math.Floor(rng() * 1e9)),
	}
}

// ─── Single-slot LRU cache ───────────────────────────────────────────────────
// Called from three hot paths (500ms sprite tick, per-keystroke PromptInput,
// per-turn observer) with the same userId → cache the deterministic result.

var (
	rollCacheMu sync.Mutex
	rollCacheK  string
	rollCacheV  Roll
)

// RollCompanion returns the deterministic Roll for a userId.
func RollCompanion(userID string) Roll {
	key := userID + Salt

	rollCacheMu.Lock()
	defer rollCacheMu.Unlock()

	if rollCacheK == key {
		return rollCacheV
	}
	value := rollFrom(mulberry32Float(hashString(key)))
	rollCacheK = key
	rollCacheV = value
	return value
}

// RollWithSeed returns a Roll from an arbitrary seed string (no cache).
func RollWithSeed(seed string) Roll {
	return rollFrom(mulberry32Float(hashString(seed)))
}

// ─── Companion assembly ──────────────────────────────────────────────────────

// GetCompanion regenerates bones from userID and merges with stored soul.
// Returns nil if no companion has been hatched yet.
// Bones never persist so species renames and SPECIES-array edits can't break
// stored companions, and editing config can't fake a rarity.
func GetCompanion(userID string, stored *StoredCompanion) *Companion {
	if stored == nil {
		return nil
	}
	r := RollCompanion(userID)
	return &Companion{
		CompanionBones: r.Bones,
		CompanionSoul:  stored.CompanionSoul,
		HatchedAt:      stored.HatchedAt,
	}
}

// ─── Utility ─────────────────────────────────────────────────────────────────

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
