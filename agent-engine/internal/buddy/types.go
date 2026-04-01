package buddy

// Species represents the 18 companion species.
type Species string

const (
	SpeciesDragon    Species = "dragon"
	SpeciesPhoenix   Species = "phoenix"
	SpeciesUnicorn   Species = "unicorn"
	SpeciesGriffin   Species = "griffin"
	SpeciesFox       Species = "fox"
	SpeciesOwl       Species = "owl"
	SpeciesCat       Species = "cat"
	SpeciesDog       Species = "dog"
	SpeciesRabbit    Species = "rabbit"
	SpeciesBear      Species = "bear"
	SpeciesWolf      Species = "wolf"
	SpeciesTiger     Species = "tiger"
	SpeciesLion      Species = "lion"
	SpeciesPanda     Species = "panda"
	SpeciesPenguin   Species = "penguin"
	SpeciesParrot    Species = "parrot"
	SpeciesTurtle    Species = "turtle"
	SpeciesDragonfly Species = "dragonfly"
)

// AllSpecies is the ordered list of all 18 species (used by PRNG selection).
var AllSpecies = []Species{
	SpeciesDragon, SpeciesPhoenix, SpeciesUnicorn, SpeciesGriffin,
	SpeciesFox, SpeciesOwl, SpeciesCat, SpeciesDog,
	SpeciesRabbit, SpeciesBear, SpeciesWolf, SpeciesTiger,
	SpeciesLion, SpeciesPanda, SpeciesPenguin, SpeciesParrot,
	SpeciesTurtle, SpeciesDragonfly,
}

// Rarity represents the 5 companion rarity tiers.
type Rarity string

const (
	RarityCommon    Rarity = "common"
	RarityUncommon  Rarity = "uncommon"
	RarityRare      Rarity = "rare"
	RarityEpic      Rarity = "epic"
	RarityLegendary Rarity = "legendary"
)

// rarityThresholds maps cumulative probability [0,1) → Rarity.
// common: 0–0.50, uncommon: 0.50–0.75, rare: 0.75–0.90,
// epic: 0.90–0.97, legendary: 0.97–1.00
var rarityThresholds = []struct {
	threshold float64
	rarity    Rarity
}{
	{0.50, RarityCommon},
	{0.75, RarityUncommon},
	{0.90, RarityRare},
	{0.97, RarityEpic},
	{1.00, RarityLegendary},
}

// RarityFromFloat selects a Rarity from a pseudo-random float in [0,1).
func RarityFromFloat(f float64) Rarity {
	for _, rt := range rarityThresholds {
		if f < rt.threshold {
			return rt.rarity
		}
	}
	return RarityLegendary
}

// CompanionBones holds the deterministic, seed-derived traits of a companion.
type CompanionBones struct {
	Species    Species `json:"species"`
	Rarity     Rarity  `json:"rarity"`
	PrimaryHue float64 `json:"primary_hue"`   // [0,360)
	PatternIdx int     `json:"pattern_idx"`   // selects markings pattern
}

// CompanionSoul holds the mutable, persisted personality state.
type CompanionSoul struct {
	Name      string  `json:"name"`
	Mood      float64 `json:"mood"`   // [0,1]
	Energy    float64 `json:"energy"` // [0,1]
	Affection float64 `json:"affection"` // [0,1]
}

// Companion is the complete companion state (bones + soul).
type Companion struct {
	Bones CompanionBones `json:"bones"`
	Soul  CompanionSoul  `json:"soul"`
}

// GenerateBones uses the Mulberry32 PRNG seeded with seed to deterministically
// derive companion traits.
func GenerateBones(seed uint32) CompanionBones {
	rng := newMulberry32(seed)
	next := func() float64 { return float64(rng.next()) / float64(^uint32(0)) }

	speciesIdx := int(rng.next()) % len(AllSpecies)
	return CompanionBones{
		Species:    AllSpecies[speciesIdx],
		Rarity:     RarityFromFloat(next()),
		PrimaryHue: next() * 360,
		PatternIdx: int(rng.next()) % 8,
	}
}
