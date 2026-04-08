package companion

import (
	"strings"
	"unicode/utf8"
)

// ─── Speech bubble renderer ──────────────────────────────────────────────────
// Renders a rounded speech bubble with auto-wrapping.
// Matches claude-code-main CompanionSprite.tsx bubble styles.
//
// Two tail modes (matching TS):
//   TailRight — inline mode: connector "─" on the right side of the last content line
//   TailDown  — float mode:  two descending "╲" lines below the bottom border

const (
	bubbleMaxWidth = 30 // max chars per line inside bubble (TS: wrap(text, 30))
	BubbleWidth    = 36 // total bubble width including borders and padding (TS BUBBLE_WIDTH = 36)
)

// TailMode selects the speech bubble tail style.
type TailMode int

const (
	TailRight TailMode = iota // inline: tail points right toward sprite
	TailDown                  // float: tail descends below bubble
)

// RenderBubble returns a speech bubble with the given text.
// If fading is true, use dimmed border style.
func RenderBubble(text string, fading bool, tail TailMode) []string {
	if text == "" {
		return nil
	}

	// Wrap text into lines
	words := strings.Fields(text)
	var lines []string
	current := ""
	for _, w := range words {
		if current == "" {
			current = w
		} else if utf8.RuneCountInString(current)+1+utf8.RuneCountInString(w) <= bubbleMaxWidth {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}

	// Find widest line (rune count for proper Unicode width)
	maxW := 0
	for _, l := range lines {
		w := utf8.RuneCountInString(l)
		if w > maxW {
			maxW = w
		}
	}
	if maxW < 3 {
		maxW = 3
	}

	// Border characters
	var out []string
	tl, tr, bl, br, h, v := "╭", "╮", "╰", "╯", "─", "│"
	if fading {
		tl, tr, bl, br, h, v = "╭", "╮", "╰", "╯", "─", "│"
		// TS fading only changes borderColor to "inactive", not the characters.
		// The fading effect is handled by the caller via lipgloss dim styling.
	}

	// Top border
	out = append(out, tl+strings.Repeat(h, maxW+2)+tr)

	// Content lines
	for _, l := range lines {
		pad := maxW - utf8.RuneCountInString(l)
		out = append(out, v+" "+l+strings.Repeat(" ", pad)+" "+v)
	}

	// Bottom border
	out = append(out, bl+strings.Repeat(h, maxW+2)+br)

	// Tail
	switch tail {
	case TailRight:
		// Append connector "─" to the right of the last content line.
		// Handled by caller when joining bubble + sprite horizontally.
	case TailDown:
		// Two descending backslash lines below the bottom border.
		indent := maxW // right-aligned under the box
		out = append(out, strings.Repeat(" ", indent)+"  ╲")
		out = append(out, strings.Repeat(" ", indent)+"   ╲")
	}

	return out
}

// BubbleWidth returns the total width of a rendered bubble (for layout).
func BubbleBoxWidth(text string) int {
	if text == "" {
		return 0
	}
	return BubbleWidth
}

// CompanionReservedColumns returns how many columns the companion reserves
// from the input area.
//
// spriteColWidth: actual column width of the sprite area (from Model.SpriteColWidth)
// speaking: whether the companion is currently speaking
// fullscreen: true if the app is in fullscreen mode (no inline bubble)
func CompanionReservedColumns(termCols int, speaking bool, spriteColWidth int, fullscreen bool) int {
	if termCols < MinColsFull {
		return 0 // narrow mode uses inline face, no column reservation
	}
	const sprPaddingX = 2
	base := spriteColWidth + sprPaddingX
	if speaking && !fullscreen {
		base += BubbleWidth
	}
	return base
}
