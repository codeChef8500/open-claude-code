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
	if r := CompanionReservedColumns(50, true, 5, false); r != 0 {
		t.Errorf("expected 0 for narrow terminal, got %d", r)
	}
}

func TestCompanionReservedColumns_Wide_NotSpeaking(t *testing.T) {
	r := CompanionReservedColumns(120, false, 5, false)
	// spriteW = max(12, 5+2=7, 16) = 16, + 2 padding = 18, no bubble
	if r != 18 {
		t.Errorf("expected 18, got %d", r)
	}
}

func TestCompanionReservedColumns_Wide_Speaking(t *testing.T) {
	r := CompanionReservedColumns(120, true, 5, false)
	// 18 + BubbleWidth(36) = 54
	if r != 54 {
		t.Errorf("expected 54, got %d", r)
	}
}

func TestCompanionReservedColumns_Fullscreen_Speaking(t *testing.T) {
	r := CompanionReservedColumns(120, true, 5, true)
	// Fullscreen suppresses inline bubble → just 18
	if r != 18 {
		t.Errorf("expected 18 (fullscreen suppresses bubble), got %d", r)
	}
}

func TestCompanionReservedColumns_LongName(t *testing.T) {
	r := CompanionReservedColumns(120, false, 15, false)
	// spriteW = max(12, 15+2=17) = 17, + 2 = 19
	if r != 19 {
		t.Errorf("expected 19 for long name, got %d", r)
	}
}
