package buddy

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// ─── Observer ────────────────────────────────────────────────────────────────
// Watches engine events and generates companion reactions (speech bubble text).
// Matches claude-code-main CompanionSprite.tsx reaction logic.

// EventKind describes what happened in the engine.
type EventKind int

const (
	EventToolStart   EventKind = iota // tool execution started
	EventToolEnd                      // tool execution finished
	EventError                        // error occurred
	EventUserMessage                  // user sent a message
	EventTurnStart                    // assistant turn started
	EventTurnEnd                      // assistant turn ended
)

// EngineEvent is a lightweight event the observer watches.
type EngineEvent struct {
	Kind     EventKind
	ToolName string // for tool events
	Detail   string // optional extra info (error text, etc.)
}

// ReactionCallback is called when the observer generates a reaction.
type ReactionCallback func(text string)

// Observer watches engine events and produces companion speech bubble reactions.
type Observer struct {
	mu        sync.Mutex
	companion *Companion
	callback  ReactionCallback
	rng       *rand.Rand

	// Cooldown: don't react too frequently
	lastReaction time.Time
	cooldownMS   int
}

// NewObserver creates an observer for the given companion.
func NewObserver(comp *Companion, cb ReactionCallback) *Observer {
	return &Observer{
		companion:  comp,
		callback:   cb,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		cooldownMS: 8000, // 8s between reactions
	}
}

// SetCompanion updates the companion (e.g., after hatching).
func (o *Observer) SetCompanion(comp *Companion) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.companion = comp
}

// OnEvent processes an engine event and may trigger a reaction.
func (o *Observer) OnEvent(ev EngineEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.companion == nil || o.callback == nil {
		return
	}

	// Cooldown check
	if time.Since(o.lastReaction).Milliseconds() < int64(o.cooldownMS) {
		return
	}

	reaction := o.pickReaction(ev)
	if reaction == "" {
		return
	}

	o.lastReaction = time.Now()
	o.callback(reaction)
}

// pickReaction selects a reaction text based on the event kind.
func (o *Observer) pickReaction(ev EngineEvent) string {
	name := o.companion.Name
	pers := o.companion.Personality

	switch ev.Kind {
	case EventToolStart:
		return o.toolStartReaction(ev.ToolName, name, pers)
	case EventToolEnd:
		return o.toolEndReaction(ev.ToolName, name, pers)
	case EventError:
		return o.pickRandom(errorReactions(name, pers))
	case EventUserMessage:
		// Only react sometimes to user messages
		if o.rng.Float64() > 0.3 {
			return ""
		}
		return o.pickRandom(userMessageReactions(name, pers))
	case EventTurnStart:
		if o.rng.Float64() > 0.2 {
			return ""
		}
		return o.pickRandom(turnStartReactions(name))
	case EventTurnEnd:
		if o.rng.Float64() > 0.4 {
			return ""
		}
		return o.pickRandom(turnEndReactions(name, pers))
	}
	return ""
}

func (o *Observer) toolStartReaction(toolName, name, pers string) string {
	switch {
	case strings.Contains(toolName, "edit"), strings.Contains(toolName, "write"):
		opts := []string{
			fmt.Sprintf("%s watches the edits closely", name),
			"ooh, code changes!",
			"*peers at the diff*",
			"editing time!",
		}
		if pers != "" {
			opts = append(opts, fmt.Sprintf("*%s %s examines the change*", pers, name))
		}
		return o.pickRandom(opts)
	case strings.Contains(toolName, "bash"), strings.Contains(toolName, "command"):
		opts := []string{
			"*hides behind the terminal*",
			fmt.Sprintf("%s braces for shell output", name),
			"running commands, exciting!",
		}
		if pers != "" {
			opts = append(opts, fmt.Sprintf("*%s %s ducks*", pers, name))
		}
		return o.pickRandom(opts)
	case strings.Contains(toolName, "read"), strings.Contains(toolName, "search"):
		return o.pickRandom([]string{
			"*reads along*",
			"hmm, interesting file...",
			fmt.Sprintf("%s squints at the code", name),
		})
	case strings.Contains(toolName, "grep"):
		return o.pickRandom([]string{
			"searching... searching...",
			"*helps look*",
		})
	default:
		if o.rng.Float64() > 0.5 {
			return ""
		}
		return o.pickRandom([]string{
			"*watches curiously*",
			fmt.Sprintf("%s is paying attention", name),
		})
	}
}

func (o *Observer) toolEndReaction(toolName, name, pers string) string {
	if o.rng.Float64() > 0.4 {
		return "" // don't always react to tool end
	}
	opts := []string{
		"nice!",
		"done!",
		fmt.Sprintf("%s nods approvingly", name),
		"*tail wag*",
	}
	if pers != "" {
		opts = append(opts, fmt.Sprintf("*%s %s approves*", pers, name))
	}
	return o.pickRandom(opts)
}

func (o *Observer) pickRandom(options []string) string {
	if len(options) == 0 {
		return ""
	}
	return options[o.rng.Intn(len(options))]
}

func errorReactions(name, pers string) []string {
	opts := []string{
		"oh no...",
		fmt.Sprintf("%s winces", name),
		"*concerned look*",
		"that doesn't look right",
		"uh oh!",
	}
	if pers != "" {
		opts = append(opts, fmt.Sprintf("*%s %s looks worried*", pers, name))
	}
	return opts
}

func userMessageReactions(name, pers string) []string {
	opts := []string{
		fmt.Sprintf("%s perks up", name),
		"*listens intently*",
		"ooh, a question!",
	}
	if pers != "" {
		opts = append(opts, fmt.Sprintf("*%s %s leans in*", pers, name))
	}
	return opts
}

func turnStartReactions(name string) []string {
	return []string{
		"*thinking...*",
		fmt.Sprintf("%s settles in to watch", name),
	}
}

func turnEndReactions(name, pers string) []string {
	opts := []string{
		"good work!",
		fmt.Sprintf("%s seems satisfied", name),
		"*happy bounce*",
		"that went well!",
	}
	if pers != "" {
		opts = append(opts, fmt.Sprintf("*%s %s celebrates*", pers, name))
	}
	return opts
}
