package askquestion

import (
	"strings"
	"testing"
)

func TestRenderPreviewBox_Basic(t *testing.T) {
	result := RenderPreviewBox(PreviewBoxOpts{
		Content:  "Hello world\nLine two",
		MaxWidth: 40,
		MinWidth: 20,
	}, DefaultPreviewBoxStyles())

	if !strings.Contains(result, "Hello world") {
		t.Error("should contain content")
	}
	if !strings.Contains(result, "┌") || !strings.Contains(result, "└") {
		t.Error("should have box borders")
	}
}

func TestRenderPreviewBox_Truncation(t *testing.T) {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, "line content here")
	}
	content := strings.Join(lines, "\n")

	result := RenderPreviewBox(PreviewBoxOpts{
		Content:  content,
		MaxLines: 10,
		MaxWidth: 40,
		MinWidth: 20,
	}, DefaultPreviewBoxStyles())

	if !strings.Contains(result, "hidden") {
		t.Error("should show truncation indicator with hidden lines")
	}
	if !strings.Contains(result, "✂") {
		t.Error("should show scissors indicator")
	}
}

func TestRenderPreviewBox_MinHeight(t *testing.T) {
	result := RenderPreviewBox(PreviewBoxOpts{
		Content:   "short",
		MinHeight: 5,
		MaxWidth:  40,
		MinWidth:  20,
	}, DefaultPreviewBoxStyles())

	lines := strings.Split(result, "\n")
	// At least minHeight content lines + 2 border lines
	if len(lines) < 7 {
		t.Errorf("expected at least 7 lines (5 content + 2 border), got %d", len(lines))
	}
}

func TestPadOrTruncateLine(t *testing.T) {
	// Short line → padded
	result := padOrTruncateLine("hi", 10)
	if len(result) != 10 {
		t.Errorf("expected length 10, got %d: %q", len(result), result)
	}

	// Exact length
	result = padOrTruncateLine("1234567890", 10)
	if result != "1234567890" {
		t.Errorf("expected exact match, got %q", result)
	}
}
