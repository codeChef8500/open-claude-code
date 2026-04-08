package companion

import (
	"strings"
	"testing"
)

func TestRenderBubble_Empty(t *testing.T) {
	lines := RenderBubble("", false, TailRight)
	if lines != nil {
		t.Error("expected nil for empty text")
	}
}

func TestRenderBubble_TailRight_HasBorder(t *testing.T) {
	lines := RenderBubble("hello", false, TailRight)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (top, content, bottom), got %d", len(lines))
	}
	// Top border starts with ╭
	if !strings.HasPrefix(lines[0], "╭") {
		t.Errorf("top border should start with ╭: %q", lines[0])
	}
	// Bottom border starts with ╰
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "╰") {
		t.Errorf("bottom border should start with ╰: %q", last)
	}
	// No tail lines appended for TailRight
	if len(lines) != 3 {
		t.Errorf("TailRight should have exactly 3 lines for single-word text, got %d", len(lines))
	}
}

func TestRenderBubble_TailDown_HasTail(t *testing.T) {
	lines := RenderBubble("hello", false, TailDown)
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 lines (top, content, bottom, 2 tail), got %d", len(lines))
	}
	// Last two lines should contain ╲
	if !strings.Contains(lines[len(lines)-2], "╲") {
		t.Errorf("tail line 1 should contain ╲: %q", lines[len(lines)-2])
	}
	if !strings.Contains(lines[len(lines)-1], "╲") {
		t.Errorf("tail line 2 should contain ╲: %q", lines[len(lines)-1])
	}
}

func TestRenderBubble_WordWrap(t *testing.T) {
	// A long text should be wrapped
	long := strings.Repeat("word ", 20)
	lines := RenderBubble(long, false, TailRight)
	// Should have more than 3 lines (top + multiple content + bottom)
	if len(lines) <= 3 {
		t.Errorf("long text should wrap into multiple lines, got %d lines", len(lines))
	}
}

func TestRenderBubble_Fading_SameBorders(t *testing.T) {
	// Fading should use same border chars (TS only changes borderColor, not chars)
	normal := RenderBubble("test", false, TailRight)
	faded := RenderBubble("test", true, TailRight)
	if len(normal) != len(faded) {
		t.Fatalf("fading should not change line count: %d vs %d", len(normal), len(faded))
	}
	// Border characters should be the same
	if normal[0] != faded[0] {
		t.Errorf("fading should not change border chars: %q vs %q", normal[0], faded[0])
	}
}

func TestBubbleBoxWidth_Empty(t *testing.T) {
	if w := BubbleBoxWidth(""); w != 0 {
		t.Errorf("expected 0 for empty, got %d", w)
	}
}

func TestBubbleBoxWidth_NonEmpty(t *testing.T) {
	if w := BubbleBoxWidth("hello"); w != BubbleWidth {
		t.Errorf("expected %d, got %d", BubbleWidth, w)
	}
}

func TestCompanionReservedColumns_Narrow(t *testing.T) {
	// Below MinColsFull → 0
	if r := CompanionReservedColumns(50, true, 12, false); r != 0 {
		t.Errorf("expected 0 for narrow terminal, got %d", r)
	}
}

func TestCompanionReservedColumns_Wide_NotSpeaking(t *testing.T) {
	// spriteColWidth=12, + 2 padding = 14, no bubble
	r := CompanionReservedColumns(120, false, 12, false)
	if r != 14 {
		t.Errorf("expected 14, got %d", r)
	}
}

func TestCompanionReservedColumns_Wide_Speaking(t *testing.T) {
	// spriteColWidth=12, + 2 padding + BubbleWidth(36) = 50
	r := CompanionReservedColumns(120, true, 12, false)
	if r != 50 {
		t.Errorf("expected 50, got %d", r)
	}
}

func TestCompanionReservedColumns_Fullscreen_Speaking(t *testing.T) {
	// Fullscreen suppresses inline bubble → just spriteColWidth(12) + 2 = 14
	r := CompanionReservedColumns(120, true, 12, true)
	if r != 14 {
		t.Errorf("expected 14 (fullscreen suppresses bubble), got %d", r)
	}
}

func TestCompanionReservedColumns_WideSpriteCol(t *testing.T) {
	// spriteColWidth=17 (long name/info row), + 2 padding = 19
	r := CompanionReservedColumns(120, false, 17, false)
	if r != 19 {
		t.Errorf("expected 19, got %d", r)
	}
}

func TestBubbleMaxWidth_MatchesTS(t *testing.T) {
	// TS wraps at 30 chars. A line of exactly 30 chars should fit in one line.
	text := "aaaaaaaaa bbbbbbbbb ccccccccc" // 29 chars, fits in one wrap line
	lines := RenderBubble(text, false, TailRight)
	// Should be exactly 3 lines: top border, 1 content line, bottom border
	if len(lines) != 3 {
		t.Errorf("expected 3 lines for text within bubbleMaxWidth, got %d", len(lines))
	}
}

func TestBubbleMaxWidth_WrapsLongLine(t *testing.T) {
	// A text that exceeds 30 chars should wrap
	text := "aaaaaaaaa bbbbbbbbb ccccccccc ddddddddd" // 39 chars
	lines := RenderBubble(text, false, TailRight)
	// Should be 4 lines: top border, 2 content lines, bottom border
	if len(lines) != 4 {
		t.Errorf("expected 4 lines for wrapped text, got %d", len(lines))
	}
}

func TestCenterText_Unicode(t *testing.T) {
	// Unicode characters should be counted by rune, not byte
	result := centerText("★★★", 12)
	// 3 runes → pad = (12-3)/2 = 4 spaces
	if result != "    ★★★" {
		t.Errorf("centerText unicode: got %q", result)
	}
}

func TestCenterText_ASCII(t *testing.T) {
	result := centerText("hi", 10)
	// 2 chars → pad = (10-2)/2 = 4 spaces
	if result != "    hi" {
		t.Errorf("centerText ascii: got %q", result)
	}
}

func TestCenterText_OverWidth(t *testing.T) {
	result := centerText("long string here", 5)
	// Wider than width → returned as-is
	if result != "long string here" {
		t.Errorf("centerText over-width: got %q", result)
	}
}
