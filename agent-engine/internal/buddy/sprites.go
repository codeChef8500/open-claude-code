package buddy

import "strings"

// ─── Sprite data ─────────────────────────────────────────────────────────────
// Each sprite is 5 lines tall, 12 wide (after {E}→1char substitution).
// Multiple frames per species for idle fidget animation.
// Line 0 is the hat slot — must be blank in frames 0-1; frame 2 may use it.
// Matches claude-code-main sprites.ts BODIES exactly.

var bodies = map[Species][][]string{
	SpeciesDuck: {
		{
			"            ",
			"    __      ",
			"  <({E} )___  ",
			"   (  ._>   ",
			"    `--´    ",
		},
		{
			"            ",
			"    __      ",
			"  <({E} )___  ",
			"   (  ._>   ",
			"    `--´~   ",
		},
		{
			"            ",
			"    __      ",
			"  <({E} )___  ",
			"   (  .__>  ",
			"    `--´    ",
		},
	},
	SpeciesGoose: {
		{
			"            ",
			"     ({E}>    ",
			"     ||     ",
			"   _(__)_   ",
			"    ^^^^    ",
		},
		{
			"            ",
			"    ({E}>     ",
			"     ||     ",
			"   _(__)_   ",
			"    ^^^^    ",
		},
		{
			"            ",
			"     ({E}>>   ",
			"     ||     ",
			"   _(__)_   ",
			"    ^^^^    ",
		},
	},
	SpeciesBlob: {
		{
			"            ",
			"   .----.   ",
			"  ( {E}  {E} )  ",
			"  (      )  ",
			"   `----´   ",
		},
		{
			"            ",
			"  .------.  ",
			" (  {E}  {E}  ) ",
			" (        ) ",
			"  `------´  ",
		},
		{
			"            ",
			"    .--.    ",
			"   ({E}  {E})   ",
			"   (    )   ",
			"    `--´    ",
		},
	},
	SpeciesCat: {
		{
			"            ",
			"   /\\_/\\    ",
			"  ( {E}   {E})  ",
			"  (  ω  )   ",
			"  (\")_(\")   ",
		},
		{
			"            ",
			"   /\\_/\\    ",
			"  ( {E}   {E})  ",
			"  (  ω  )   ",
			"  (\")_(\")~  ",
		},
		{
			"            ",
			"   /\\-/\\    ",
			"  ( {E}   {E})  ",
			"  (  ω  )   ",
			"  (\")_(\")   ",
		},
	},
	SpeciesDragon: {
		{
			"            ",
			"  /^\\  /^\\  ",
			" <  {E}  {E}  > ",
			" (   ~~   ) ",
			"  `-vvvv-´  ",
		},
		{
			"            ",
			"  /^\\  /^\\  ",
			" <  {E}  {E}  > ",
			" (        ) ",
			"  `-vvvv-´  ",
		},
		{
			"   ~    ~   ",
			"  /^\\  /^\\  ",
			" <  {E}  {E}  > ",
			" (   ~~   ) ",
			"  `-vvvv-´  ",
		},
	},
	SpeciesOctopus: {
		{
			"            ",
			"   .----.   ",
			"  ( {E}  {E} )  ",
			"  (______)  ",
			"  /\\/\\/\\/\\  ",
		},
		{
			"            ",
			"   .----.   ",
			"  ( {E}  {E} )  ",
			"  (______)  ",
			"  \\/\\/\\/\\/  ",
		},
		{
			"     o      ",
			"   .----.   ",
			"  ( {E}  {E} )  ",
			"  (______)  ",
			"  /\\/\\/\\/\\  ",
		},
	},
	SpeciesOwl: {
		{
			"            ",
			"   /\\  /\\   ",
			"  (({E})({E}))  ",
			"  (  ><  )  ",
			"   `----´   ",
		},
		{
			"            ",
			"   /\\  /\\   ",
			"  (({E})({E}))  ",
			"  (  ><  )  ",
			"   .----.   ",
		},
		{
			"            ",
			"   /\\  /\\   ",
			"  (({E})(-))  ",
			"  (  ><  )  ",
			"   `----´   ",
		},
	},
	SpeciesPenguin: {
		{
			"            ",
			"  .---.     ",
			"  ({E}>{E})     ",
			" /(   )\\    ",
			"  `---´     ",
		},
		{
			"            ",
			"  .---.     ",
			"  ({E}>{E})     ",
			" |(   )|    ",
			"  `---´     ",
		},
		{
			"  .---.     ",
			"  ({E}>{E})     ",
			" /(   )\\    ",
			"  `---´     ",
			"   ~ ~      ",
		},
	},
	SpeciesTurtle: {
		{
			"            ",
			"   _,--._   ",
			"  ( {E}  {E} )  ",
			" /[______]\\ ",
			"  ``    ``  ",
		},
		{
			"            ",
			"   _,--._   ",
			"  ( {E}  {E} )  ",
			" /[______]\\ ",
			"   ``  ``   ",
		},
		{
			"            ",
			"   _,--._   ",
			"  ( {E}  {E} )  ",
			" /[======]\\ ",
			"  ``    ``  ",
		},
	},
	SpeciesSnail: {
		{
			"            ",
			" {E}    .--.  ",
			"  \\  ( @ )  ",
			"   \\_`--´   ",
			"  ~~~~~~~   ",
		},
		{
			"            ",
			"  {E}   .--.  ",
			"  |  ( @ )  ",
			"   \\_`--´   ",
			"  ~~~~~~~   ",
		},
		{
			"            ",
			" {E}    .--.  ",
			"  \\  ( @  ) ",
			"   \\_`--´   ",
			"   ~~~~~~   ",
		},
	},
	SpeciesGhost: {
		{
			"            ",
			"   .----.   ",
			"  / {E}  {E} \\  ",
			"  |      |  ",
			"  ~`~``~`~  ",
		},
		{
			"            ",
			"   .----.   ",
			"  / {E}  {E} \\  ",
			"  |      |  ",
			"  `~`~~`~`  ",
		},
		{
			"    ~  ~    ",
			"   .----.   ",
			"  / {E}  {E} \\  ",
			"  |      |  ",
			"  ~~`~~`~~  ",
		},
	},
	SpeciesAxolotl: {
		{
			"            ",
			"}~(______)~{",
			"}~({E} .. {E})~{",
			"  ( .--. )  ",
			"  (_/  \\_)  ",
		},
		{
			"            ",
			"~}(______){~",
			"~}({E} .. {E}){~",
			"  ( .--. )  ",
			"  (_/  \\_)  ",
		},
		{
			"            ",
			"}~(______)~{",
			"}~({E} .. {E})~{",
			"  (  --  )  ",
			"  ~_/  \\_~  ",
		},
	},
	SpeciesCapybara: {
		{
			"            ",
			"  n______n  ",
			" ( {E}    {E} ) ",
			" (   oo   ) ",
			"  `------´  ",
		},
		{
			"            ",
			"  n______n  ",
			" ( {E}    {E} ) ",
			" (   Oo   ) ",
			"  `------´  ",
		},
		{
			"    ~  ~    ",
			"  u______n  ",
			" ( {E}    {E} ) ",
			" (   oo   ) ",
			"  `------´  ",
		},
	},
	SpeciesCactus: {
		{
			"            ",
			" n  ____  n ",
			" | |{E}  {E}| | ",
			" |_|    |_| ",
			"   |    |   ",
		},
		{
			"            ",
			"    ____    ",
			" n |{E}  {E}| n ",
			" |_|    |_| ",
			"   |    |   ",
		},
		{
			" n        n ",
			" |  ____  | ",
			" | |{E}  {E}| | ",
			" |_|    |_| ",
			"   |    |   ",
		},
	},
	SpeciesRobot: {
		{
			"            ",
			"   .[||].   ",
			"  [ {E}  {E} ]  ",
			"  [ ==== ]  ",
			"  `------´  ",
		},
		{
			"            ",
			"   .[||].   ",
			"  [ {E}  {E} ]  ",
			"  [ -==- ]  ",
			"  `------´  ",
		},
		{
			"     *      ",
			"   .[||].   ",
			"  [ {E}  {E} ]  ",
			"  [ ==== ]  ",
			"  `------´  ",
		},
	},
	SpeciesRabbit: {
		{
			"            ",
			"   (\\__/)   ",
			"  ( {E}  {E} )  ",
			" =(  ..  )= ",
			"  (\")__(\")  ",
		},
		{
			"            ",
			"   (|__/)   ",
			"  ( {E}  {E} )  ",
			" =(  ..  )= ",
			"  (\")__(\")  ",
		},
		{
			"            ",
			"   (\\__/)   ",
			"  ( {E}  {E} )  ",
			" =( .  . )= ",
			"  (\")__(\")  ",
		},
	},
	SpeciesMushroom: {
		{
			"            ",
			" .-o-OO-o-. ",
			"(__________)",
			"   |{E}  {E}|   ",
			"   |____|   ",
		},
		{
			"            ",
			" .-O-oo-O-. ",
			"(__________)",
			"   |{E}  {E}|   ",
			"   |____|   ",
		},
		{
			"   . o  .   ",
			" .-o-OO-o-. ",
			"(__________)",
			"   |{E}  {E}|   ",
			"   |____|   ",
		},
	},
	SpeciesChonk: {
		{
			"            ",
			"  /\\    /\\  ",
			" ( {E}    {E} ) ",
			" (   ..   ) ",
			"  `------´  ",
		},
		{
			"            ",
			"  /\\    /|  ",
			" ( {E}    {E} ) ",
			" (   ..   ) ",
			"  `------´  ",
		},
		{
			"            ",
			"  /\\    /\\  ",
			" ( {E}    {E} ) ",
			" (   ..   ) ",
			"  `------´~ ",
		},
	},
}

// hatLines maps hat type to the ASCII hat line that replaces line 0.
var hatLines = map[Hat]string{
	HatNone:      "",
	HatCrown:     "   \\^^^/    ",
	HatTophat:    "   [___]    ",
	HatPropeller: "    -+-     ",
	HatHalo:      "   (   )    ",
	HatWizard:    "    /^\\     ",
	HatBeanie:    "   (___)    ",
	HatTinyDuck:  "    ,>      ",
}

// RenderSprite returns the ASCII lines for a companion at the given frame.
// {E} placeholders are replaced with the actual eye character.
// Hat is overlaid on line 0 if the line is blank and hat != none.
// Blank hat-slot line is dropped when ALL frames have blank line 0.
func RenderSprite(bones CompanionBones, frame int) []string {
	frames, ok := bodies[bones.Species]
	if !ok {
		return []string{"  ???  "}
	}
	src := frames[frame%len(frames)]
	lines := make([]string, len(src))
	for i, line := range src {
		lines[i] = strings.ReplaceAll(line, "{E}", string(bones.Eye))
	}

	// Overlay hat on line 0 if blank
	if bones.Hat != HatNone && strings.TrimSpace(lines[0]) == "" {
		if hl, ok := hatLines[bones.Hat]; ok && hl != "" {
			lines[0] = hl
		}
	}

	// Drop blank hat slot only if ALL frames have blank line 0
	allBlank := true
	for _, f := range frames {
		if strings.TrimSpace(f[0]) != "" {
			allBlank = false
			break
		}
	}
	if allBlank && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}

	return lines
}

// SpriteFrameCount returns the number of animation frames for a species.
func SpriteFrameCount(species Species) int {
	if frames, ok := bodies[species]; ok {
		return len(frames)
	}
	return 1
}

// RenderFace returns a single-line face for narrow terminals.
// Matches claude-code-main sprites.ts renderFace() switch-case exactly.
func RenderFace(bones CompanionBones) string {
	e := string(bones.Eye)
	switch bones.Species {
	case SpeciesDuck, SpeciesGoose:
		return "(" + e + ">"
	case SpeciesBlob:
		return "(" + e + e + ")"
	case SpeciesCat:
		return "=" + e + "ω" + e + "="
	case SpeciesDragon:
		return "<" + e + "~" + e + ">"
	case SpeciesOctopus:
		return "~(" + e + e + ")~"
	case SpeciesOwl:
		return "(" + e + ")(" + e + ")"
	case SpeciesPenguin:
		return "(" + e + ">)"
	case SpeciesTurtle:
		return "[" + e + "_" + e + "]"
	case SpeciesSnail:
		return e + "(@)"
	case SpeciesGhost:
		return "/" + e + e + "\\"
	case SpeciesAxolotl:
		return "}" + e + "." + e + "{"
	case SpeciesCapybara:
		return "(" + e + "oo" + e + ")"
	case SpeciesCactus:
		return "|" + e + "  " + e + "|"
	case SpeciesRobot:
		return "[" + e + e + "]"
	case SpeciesRabbit:
		return "(" + e + ".." + e + ")"
	case SpeciesMushroom:
		return "|" + e + "  " + e + "|"
	case SpeciesChonk:
		return "(" + e + "." + e + ")"
	default:
		return "(" + e + e + ")"
	}
}
