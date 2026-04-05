package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /buddy ──────────────────────────────────────────────────────────────────
// Replaces /hatch. Supports subcommands: (none)=card, pet, mute, unmute, stats.
// Aligned with claude-code-main commands/buddy/index.ts.
//
// NOTE: This command does NOT import the buddy package to avoid an import cycle
// (command → buddy → engine → command). Instead, it returns well-known signal
// strings that the TUI bridge layer interprets to call buddy logic.
//
// Signal protocol:
//   __buddy_show__       — show companion card (hatch if needed)
//   __buddy_pet__        — trigger pet animation
//   __buddy_mute__       — mute companion
//   __buddy_unmute__     — unmute companion
//   __buddy_stats__      — show stats card

type BuddyCommand struct{ BaseCommand }

func (c *BuddyCommand) Name() string                  { return "buddy" }
func (c *BuddyCommand) Aliases() []string             { return []string{"companion", "hatch"} }
func (c *BuddyCommand) ArgumentHint() string          { return "[pet|mute|unmute|stats]" }
func (c *BuddyCommand) Description() string           { return "Interact with your companion buddy" }
func (c *BuddyCommand) Type() CommandType             { return CommandTypeLocal }
func (c *BuddyCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *BuddyCommand) Execute(_ context.Context, args []string, _ *ExecContext) (string, error) {
	sub := ""
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}

	switch sub {
	case "pet":
		return "__buddy_pet__", nil
	case "mute":
		return "__buddy_mute__", nil
	case "unmute":
		return "__buddy_unmute__", nil
	case "stats":
		return "__buddy_stats__", nil
	default:
		return "__buddy_show__", nil
	}
}

// BuddySignalPrefix is the prefix for all buddy command signals.
const BuddySignalPrefix = "__buddy_"

// IsBuddySignal checks if a command result is a buddy signal.
func IsBuddySignal(result string) bool {
	return strings.HasPrefix(result, BuddySignalPrefix)
}

// ParseBuddySignal extracts the action from a buddy signal string.
// Returns empty string if not a buddy signal.
func ParseBuddySignal(result string) string {
	if !IsBuddySignal(result) {
		return ""
	}
	s := strings.TrimPrefix(result, BuddySignalPrefix)
	s = strings.TrimSuffix(s, "__")
	return s
}

// BuddyCardText builds a companion card from pre-formatted parts.
// Called by the TUI bridge which has access to the buddy package.
func BuddyCardText(name, species, rarity, stars, personality string, shiny bool, hatchDate string, spriteLines []string) string {
	var sb strings.Builder
	sb.WriteString("╭────────────────────────────────╮\n")
	sb.WriteString(fmt.Sprintf("│  %s  %s\n", name, stars))
	sb.WriteString(fmt.Sprintf("│  %s %s\n", species, rarity))
	sb.WriteString("│\n")
	for _, line := range spriteLines {
		sb.WriteString(fmt.Sprintf("│  %s\n", line))
	}
	sb.WriteString("│\n")
	if personality != "" {
		sb.WriteString(fmt.Sprintf("│  \"%s\"\n", personality))
	}
	if shiny {
		sb.WriteString("│  ✨ SHINY!\n")
	}
	sb.WriteString(fmt.Sprintf("│  Hatched: %s\n", hatchDate))
	sb.WriteString("╰────────────────────────────────╯")
	return sb.String()
}

// BuddyStatsText builds a stats card from pre-formatted parts.
func BuddyStatsText(name, rarity, stars, eye, hat string, shiny bool, stats map[string]int, statOrder []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("── %s Stats ──\n\n", name))
	for _, stat := range statOrder {
		val := stats[stat]
		bar := renderStatBar(val)
		sb.WriteString(fmt.Sprintf("  %-10s %s %3d\n", stat, bar, val))
	}
	sb.WriteString(fmt.Sprintf("\n  Rarity:  %s %s\n", rarity, stars))
	sb.WriteString(fmt.Sprintf("  Eye:     %s\n", eye))
	sb.WriteString(fmt.Sprintf("  Hat:     %s\n", hat))
	if shiny {
		sb.WriteString("  ✨ SHINY!\n")
	}
	return sb.String()
}

// renderStatBar creates a visual bar ████░░░░░░ for a stat value 0-100.
func renderStatBar(val int) string {
	filled := val / 5 // 0-20 blocks
	if filled > 20 {
		filled = 20
	}
	empty := 20 - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}
