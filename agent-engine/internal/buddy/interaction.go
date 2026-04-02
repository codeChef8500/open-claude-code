package buddy

import (
	"fmt"
	"time"
)

// InteractionType classifies user interactions with a companion.
type InteractionType string

const (
	InteractionFeed InteractionType = "feed"
	InteractionPlay InteractionType = "play"
	InteractionPet  InteractionType = "pet"
	InteractionRest InteractionType = "rest"
	InteractionTask InteractionType = "task" // companion assisted with a task
)

// Interaction records a single user–companion interaction.
type Interaction struct {
	Type      InteractionType `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Delta     StatDelta       `json:"delta"`
}

// StatDelta describes the effect of an interaction on companion stats.
type StatDelta struct {
	Mood      float64 `json:"mood"`
	Energy    float64 `json:"energy"`
	Affection float64 `json:"affection"`
}

// interactionEffects maps each interaction type to its stat delta.
var interactionEffects = map[InteractionType]StatDelta{
	InteractionFeed: {Mood: 0.05, Energy: 0.15, Affection: 0.02},
	InteractionPlay: {Mood: 0.12, Energy: -0.10, Affection: 0.08},
	InteractionPet:  {Mood: 0.08, Energy: 0.0, Affection: 0.10},
	InteractionRest: {Mood: 0.03, Energy: 0.20, Affection: 0.01},
	InteractionTask: {Mood: -0.02, Energy: -0.05, Affection: 0.05},
}

// InteractionLog tracks companion interactions with cooldown enforcement.
type InteractionLog struct {
	History  []Interaction `json:"history"`
	Cooldown time.Duration `json:"-"`
}

// NewInteractionLog creates a log with a default 30-second cooldown.
func NewInteractionLog() *InteractionLog {
	return &InteractionLog{Cooldown: 30 * time.Second}
}

// CanInteract checks if enough time has passed since the last interaction.
func (log *InteractionLog) CanInteract() bool {
	if len(log.History) == 0 {
		return true
	}
	last := log.History[len(log.History)-1]
	return time.Since(last.Timestamp) >= log.Cooldown
}

// Apply performs an interaction on the companion, updating its soul stats.
// Returns the interaction record or an error if on cooldown.
func (log *InteractionLog) Apply(c *Companion, itype InteractionType) (*Interaction, error) {
	if !log.CanInteract() {
		remaining := log.Cooldown - time.Since(log.History[len(log.History)-1].Timestamp)
		return nil, fmt.Errorf("on cooldown: wait %s", remaining.Round(time.Second))
	}

	delta, ok := interactionEffects[itype]
	if !ok {
		return nil, fmt.Errorf("unknown interaction type: %s", itype)
	}

	// Apply delta to soul.
	c.Soul.Mood = clamp(c.Soul.Mood+delta.Mood, 0, 1)
	c.Soul.Energy = clamp(c.Soul.Energy+delta.Energy, 0, 1)
	c.Soul.Affection = clamp(c.Soul.Affection+delta.Affection, 0, 1)

	interaction := Interaction{
		Type:      itype,
		Timestamp: time.Now(),
		Delta:     delta,
	}
	log.History = append(log.History, interaction)

	// Keep only last 100 interactions.
	if len(log.History) > 100 {
		log.History = log.History[len(log.History)-100:]
	}

	return &interaction, nil
}

// TotalInteractions returns the number of logged interactions.
func (log *InteractionLog) TotalInteractions() int {
	return len(log.History)
}

// InteractionCounts returns a count per interaction type.
func (log *InteractionLog) InteractionCounts() map[InteractionType]int {
	counts := make(map[InteractionType]int)
	for _, i := range log.History {
		counts[i.Type]++
	}
	return counts
}

// TimeSinceLastInteraction returns how long ago the last interaction was.
func (log *InteractionLog) TimeSinceLastInteraction() time.Duration {
	if len(log.History) == 0 {
		return 0
	}
	return time.Since(log.History[len(log.History)-1].Timestamp)
}

// DecayStats applies passive stat decay based on elapsed time (called periodically).
// Energy and mood decay slowly if the companion hasn't been interacted with.
func DecayStats(c *Companion, elapsed time.Duration) {
	hours := elapsed.Hours()
	if hours <= 0 {
		return
	}
	// Decay rates per hour.
	c.Soul.Energy = clamp(c.Soul.Energy-0.02*hours, 0.1, 1)
	c.Soul.Mood = clamp(c.Soul.Mood-0.01*hours, 0.1, 1)
}
