package companion

import "time"

// ─── Animation constants ─────────────────────────────────────────────────────
// Matches claude-code-main CompanionSprite.tsx timing.

const (
	TickMS       = 500                       // ms per animation tick
	TickDuration = TickMS * time.Millisecond // as time.Duration
	BubbleShow   = 20                        // ticks to show speech bubble (~10s)
	FadeWindow   = 6                         // last N ticks: bubble fades
	PetBurstMS   = 2500                      // heart animation duration
	MinColsFull  = 100                       // min terminal cols for full sprite
)

// IdleSequence controls the idle animation loop.
// -1 = blink, 0/1/2 = frame index. 15 entries ≈ 7.5s cycle.
var IdleSequence = []int{0, 0, 0, 0, 1, 0, 0, 0, -1, 0, 0, 2, 0, 0, 0}

// Heart frames prepended above the sprite during petting.
var HeartFrames = []string{
	"   ♥    ♥   ",
	"  ♥  ♥   ♥  ",
	" ♥   ♥  ♥   ",
	"♥  ♥      ♥ ",
	"·    ·   ·  ",
}

// AnimState describes the current animation mode.
type AnimState int

const (
	AnimIdle   AnimState = iota // normal idle cycle
	AnimExcite                  // excited (reaction or petting) — fast frame cycle
	AnimBlink                   // eyes replaced with '-'
)
