package buddy

// ─── Species ─────────────────────────────────────────────────────────────────
// 18 companion species, matching claude-code-main types.ts exactly.
// Order matters: PRNG selection indexes into AllSpecies.

type Species string

const (
	SpeciesDuck     Species = "duck"
	SpeciesGoose    Species = "goose"
	SpeciesBlob     Species = "blob"
	SpeciesCat      Species = "cat"
	SpeciesDragon   Species = "dragon"
	SpeciesOctopus  Species = "octopus"
	SpeciesOwl      Species = "owl"
	SpeciesPenguin  Species = "penguin"
	SpeciesTurtle   Species = "turtle"
	SpeciesSnail    Species = "snail"
	SpeciesGhost    Species = "ghost"
	SpeciesAxolotl  Species = "axolotl"
	SpeciesCapybara Species = "capybara"
	SpeciesCactus   Species = "cactus"
	SpeciesRobot    Species = "robot"
	SpeciesRabbit   Species = "rabbit"
	SpeciesMushroom Species = "mushroom"
	SpeciesChonk    Species = "chonk"
)

// AllSpecies is the ordered list of all 18 species (PRNG indexes into this).
var AllSpecies = []Species{
	SpeciesDuck, SpeciesGoose, SpeciesBlob, SpeciesCat, SpeciesDragon,
	SpeciesOctopus, SpeciesOwl, SpeciesPenguin, SpeciesTurtle, SpeciesSnail,
	SpeciesGhost, SpeciesAxolotl, SpeciesCapybara, SpeciesCactus, SpeciesRobot,
	SpeciesRabbit, SpeciesMushroom, SpeciesChonk,
}

// ─── Rarity ──────────────────────────────────────────────────────────────────
// 5 tiers with weighted probabilities summing to 100.

type Rarity string

const (
	RarityCommon    Rarity = "common"
	RarityUncommon  Rarity = "uncommon"
	RarityRare      Rarity = "rare"
	RarityEpic      Rarity = "epic"
	RarityLegendary Rarity = "legendary"
)

// AllRarities in ascending order (used by rollRarity weighted selection).
var AllRarities = []Rarity{
	RarityCommon, RarityUncommon, RarityRare, RarityEpic, RarityLegendary,
}

// RarityWeights maps rarity → weight (total = 100).
var RarityWeights = map[Rarity]int{
	RarityCommon:    60,
	RarityUncommon:  25,
	RarityRare:      10,
	RarityEpic:      4,
	RarityLegendary: 1,
}

// RarityStars maps rarity → display stars.
var RarityStars = map[Rarity]string{
	RarityCommon:    "★",
	RarityUncommon:  "★★",
	RarityRare:      "★★★",
	RarityEpic:      "★★★★",
	RarityLegendary: "★★★★★",
}

// RarityColors maps rarity → theme color key (for TUI rendering).
var RarityColors = map[Rarity]string{
	RarityCommon:    "inactive",
	RarityUncommon:  "success",
	RarityRare:      "permission",
	RarityEpic:      "autoAccept",
	RarityLegendary: "warning",
}

// RarityHexColors maps rarity → hex color for direct TUI styling.
// Values match the dark theme: Inactive, Success, Permission, AutoAccept, Warning.
var RarityHexColors = map[Rarity]string{
	RarityCommon:    "#999999",
	RarityUncommon:  "#4eba65",
	RarityRare:      "#b1b9f9",
	RarityEpic:      "#4eba65",
	RarityLegendary: "#ffc107",
}

// RarityFloor maps rarity → base stat floor for rollStats.
var RarityFloor = map[Rarity]int{
	RarityCommon:    5,
	RarityUncommon:  15,
	RarityRare:      25,
	RarityEpic:      35,
	RarityLegendary: 50,
}

// ─── Eye ─────────────────────────────────────────────────────────────────────

type Eye string

const (
	EyeDot    Eye = "·"
	EyeStar   Eye = "✦"
	EyeCross  Eye = "×"
	EyeCircle Eye = "◉"
	EyeAt     Eye = "@"
	EyeDegree Eye = "°"
)

// AllEyes is the ordered list of eye characters.
var AllEyes = []Eye{EyeDot, EyeStar, EyeCross, EyeCircle, EyeAt, EyeDegree}

// ─── Hat ─────────────────────────────────────────────────────────────────────

type Hat string

const (
	HatNone      Hat = "none"
	HatCrown     Hat = "crown"
	HatTophat    Hat = "tophat"
	HatPropeller Hat = "propeller"
	HatHalo      Hat = "halo"
	HatWizard    Hat = "wizard"
	HatBeanie    Hat = "beanie"
	HatTinyDuck  Hat = "tinyduck"
)

// AllHats is the ordered list of hat types.
// Rule: common rarity forces HatNone; uncommon+ randomly picks from AllHats.
var AllHats = []Hat{
	HatNone, HatCrown, HatTophat, HatPropeller,
	HatHalo, HatWizard, HatBeanie, HatTinyDuck,
}

// ─── Stats ───────────────────────────────────────────────────────────────────

type StatName string

const (
	StatDebugging StatName = "DEBUGGING"
	StatPatience  StatName = "PATIENCE"
	StatChaos     StatName = "CHAOS"
	StatWisdom    StatName = "WISDOM"
	StatSnark     StatName = "SNARK"
)

// AllStatNames in defined order.
var AllStatNames = []StatName{
	StatDebugging, StatPatience, StatChaos, StatWisdom, StatSnark,
}

// ─── Type hierarchy ──────────────────────────────────────────────────────────
// Matches claude-code-main:
//   CompanionBones — deterministic, derived from hash(userId), NEVER persisted
//   CompanionSoul  — LLM-generated, persisted in config
//   Companion      — Bones + Soul + hatchedAt
//   StoredCompanion — what actually goes into config.json

// CompanionBones holds the deterministic, seed-derived traits of a companion.
// Regenerated from hash(userId + SALT) on every read — never stored.
type CompanionBones struct {
	Rarity  Rarity           `json:"rarity"`
	Species Species          `json:"species"`
	Eye     Eye              `json:"eye"`
	Hat     Hat              `json:"hat"`
	Shiny   bool             `json:"shiny"`
	Stats   map[StatName]int `json:"stats"`
}

// CompanionSoul holds the LLM-generated personality — persisted in config.
type CompanionSoul struct {
	Name        string `json:"name"`
	Personality string `json:"personality"`
}

// Companion is the complete companion state (bones + soul + hatch time).
type Companion struct {
	CompanionBones
	CompanionSoul
	HatchedAt int64 `json:"hatched_at"` // Unix milliseconds
}

// StoredCompanion is what actually persists in config.
// Bones are regenerated from hash(userId) on every read so species renames
// don't break stored companions and users can't edit their way to a legendary.
type StoredCompanion struct {
	CompanionSoul
	HatchedAt int64 `json:"hatched_at"`
}
