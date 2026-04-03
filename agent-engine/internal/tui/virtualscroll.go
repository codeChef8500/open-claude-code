package tui

import (
	"fmt"
	"strings"
)

// ────────────────────────────────────────────────────────────────────────────
// VirtualScroll — efficient rendering for large message histories.
// Only renders visible lines, keeping memory usage constant regardless
// of conversation length. Aligned with claude-code-main's virtual scroll.
// ────────────────────────────────────────────────────────────────────────────

// VirtualScrollModel manages a virtualized scrollable view of lines.
type VirtualScrollModel struct {
	// lines holds the full content split into lines.
	lines []string
	// offset is the index of the first visible line.
	offset int
	// visibleHeight is how many lines are visible at once.
	visibleHeight int
	// width is the viewport width (for wrapping).
	width int
	// totalLines is the count of all lines including wrapped.
	totalLines int
	// autoScroll if true, keeps the view pinned to the bottom.
	autoScroll bool
}

// NewVirtualScrollModel creates a virtual scroll model.
func NewVirtualScrollModel(width, height int) *VirtualScrollModel {
	return &VirtualScrollModel{
		visibleHeight: height,
		width:         width,
		autoScroll:    true,
	}
}

// SetContent replaces all content and optionally scrolls to bottom.
func (vs *VirtualScrollModel) SetContent(content string) {
	vs.lines = strings.Split(content, "\n")
	vs.totalLines = len(vs.lines)
	if vs.autoScroll {
		vs.ScrollToBottom()
	}
}

// AppendLine adds a line to the bottom and auto-scrolls if enabled.
func (vs *VirtualScrollModel) AppendLine(line string) {
	vs.lines = append(vs.lines, line)
	vs.totalLines = len(vs.lines)
	if vs.autoScroll {
		vs.ScrollToBottom()
	}
}

// AppendLines adds multiple lines.
func (vs *VirtualScrollModel) AppendLines(lines []string) {
	vs.lines = append(vs.lines, lines...)
	vs.totalLines = len(vs.lines)
	if vs.autoScroll {
		vs.ScrollToBottom()
	}
}

// ScrollUp moves the viewport up by n lines.
func (vs *VirtualScrollModel) ScrollUp(n int) {
	vs.autoScroll = false
	vs.offset -= n
	if vs.offset < 0 {
		vs.offset = 0
	}
}

// ScrollDown moves the viewport down by n lines.
func (vs *VirtualScrollModel) ScrollDown(n int) {
	vs.offset += n
	maxOffset := vs.maxOffset()
	if vs.offset >= maxOffset {
		vs.offset = maxOffset
		vs.autoScroll = true
	}
}

// ScrollToTop jumps to the beginning.
func (vs *VirtualScrollModel) ScrollToTop() {
	vs.offset = 0
	vs.autoScroll = false
}

// ScrollToBottom jumps to the end and re-enables auto-scroll.
func (vs *VirtualScrollModel) ScrollToBottom() {
	vs.offset = vs.maxOffset()
	vs.autoScroll = true
}

// PageUp moves up by one page.
func (vs *VirtualScrollModel) PageUp() {
	vs.ScrollUp(vs.visibleHeight)
}

// PageDown moves down by one page.
func (vs *VirtualScrollModel) PageDown() {
	vs.ScrollDown(vs.visibleHeight)
}

// SetSize updates the viewport dimensions.
func (vs *VirtualScrollModel) SetSize(width, height int) {
	vs.width = width
	vs.visibleHeight = height
	if vs.autoScroll {
		vs.ScrollToBottom()
	}
}

// Render returns only the visible lines as a string.
func (vs *VirtualScrollModel) Render() string {
	if len(vs.lines) == 0 {
		return ""
	}

	start := vs.offset
	end := start + vs.visibleHeight
	if end > len(vs.lines) {
		end = len(vs.lines)
	}
	if start >= end {
		if len(vs.lines) > vs.visibleHeight {
			start = len(vs.lines) - vs.visibleHeight
		} else {
			start = 0
		}
		end = len(vs.lines)
	}

	visible := vs.lines[start:end]
	return strings.Join(visible, "\n")
}

// ScrollInfo returns a human-readable scroll position indicator.
func (vs *VirtualScrollModel) ScrollInfo() string {
	if vs.totalLines <= vs.visibleHeight {
		return ""
	}
	pct := 0
	maxOff := vs.maxOffset()
	if maxOff > 0 {
		pct = vs.offset * 100 / maxOff
	}
	return fmt.Sprintf("(%d%%) %d/%d", pct, vs.offset+vs.visibleHeight, vs.totalLines)
}

// IsAtBottom reports whether the view is scrolled to the bottom.
func (vs *VirtualScrollModel) IsAtBottom() bool {
	return vs.autoScroll || vs.offset >= vs.maxOffset()
}

// TotalLines returns the total number of lines.
func (vs *VirtualScrollModel) TotalLines() int {
	return vs.totalLines
}

func (vs *VirtualScrollModel) maxOffset() int {
	max := vs.totalLines - vs.visibleHeight
	if max < 0 {
		return 0
	}
	return max
}

// ────────────────────────────────────────────────────────────────────────────
// InputHistory — tracks and navigates previous user inputs.
// ────────────────────────────────────────────────────────────────────────────

// InputHistory stores previously submitted inputs for recall.
type InputHistory struct {
	entries []string
	cursor  int
	maxSize int
	draft   string // unsaved current input
}

// NewInputHistory creates an input history buffer.
func NewInputHistory(maxSize int) *InputHistory {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &InputHistory{maxSize: maxSize, cursor: -1}
}

// Add saves a submitted input to history.
func (ih *InputHistory) Add(text string) {
	if text == "" {
		return
	}
	// Deduplicate: don't add if same as last entry.
	if len(ih.entries) > 0 && ih.entries[len(ih.entries)-1] == text {
		ih.cursor = -1
		return
	}
	ih.entries = append(ih.entries, text)
	if len(ih.entries) > ih.maxSize {
		ih.entries = ih.entries[len(ih.entries)-ih.maxSize:]
	}
	ih.cursor = -1
}

// Prev navigates to the previous entry. Returns the entry text and true,
// or empty string and false if at the beginning.
func (ih *InputHistory) Prev(currentDraft string) (string, bool) {
	if len(ih.entries) == 0 {
		return "", false
	}
	if ih.cursor == -1 {
		ih.draft = currentDraft
		ih.cursor = len(ih.entries) - 1
	} else if ih.cursor > 0 {
		ih.cursor--
	} else {
		return ih.entries[0], true
	}
	return ih.entries[ih.cursor], true
}

// Next navigates to the next entry. Returns the entry text and true,
// or the draft and false if at the end.
func (ih *InputHistory) Next() (string, bool) {
	if ih.cursor == -1 {
		return ih.draft, false
	}
	ih.cursor++
	if ih.cursor >= len(ih.entries) {
		ih.cursor = -1
		return ih.draft, false
	}
	return ih.entries[ih.cursor], true
}

// Reset clears the navigation cursor.
func (ih *InputHistory) Reset() {
	ih.cursor = -1
	ih.draft = ""
}

// All returns all history entries.
func (ih *InputHistory) All() []string {
	out := make([]string, len(ih.entries))
	copy(out, ih.entries)
	return out
}
