package session

import (
	"fmt"
	"strings"
	"time"
)

// TranscriptEntry is a single line in the transcript view.
type TranscriptEntry struct {
	Timestamp time.Time
	Role      string // user, assistant, system, tool_use, tool_result, error
	Content   string
	ToolName  string
	IsError   bool
}

// TranscriptView manages a read-only transcript of the conversation.
type TranscriptView struct {
	entries    []TranscriptEntry
	offset     int
	height     int
	width      int
	searchTerm string
	searchHits []int
}

// NewTranscriptView creates a transcript view.
func NewTranscriptView(width, height int) *TranscriptView {
	return &TranscriptView{
		width:  width,
		height: height,
	}
}

// Append adds an entry to the transcript.
func (tv *TranscriptView) Append(entry TranscriptEntry) {
	tv.entries = append(tv.entries, entry)
}

// SetSize updates dimensions.
func (tv *TranscriptView) SetSize(width, height int) {
	tv.width = width
	tv.height = height
}

// ScrollUp moves the view up.
func (tv *TranscriptView) ScrollUp(n int) {
	tv.offset -= n
	if tv.offset < 0 {
		tv.offset = 0
	}
}

// ScrollDown moves the view down.
func (tv *TranscriptView) ScrollDown(n int) {
	tv.offset += n
	max := len(tv.entries) - tv.height
	if max < 0 {
		max = 0
	}
	if tv.offset > max {
		tv.offset = max
	}
}

// Search filters entries by a search term.
func (tv *TranscriptView) Search(term string) {
	tv.searchTerm = strings.ToLower(term)
	tv.searchHits = nil
	if tv.searchTerm == "" {
		return
	}
	for i, e := range tv.entries {
		if strings.Contains(strings.ToLower(e.Content), tv.searchTerm) ||
			strings.Contains(strings.ToLower(e.ToolName), tv.searchTerm) {
			tv.searchHits = append(tv.searchHits, i)
		}
	}
	// Jump to first hit
	if len(tv.searchHits) > 0 {
		tv.offset = tv.searchHits[0]
	}
}

// ClearSearch removes search filter.
func (tv *TranscriptView) ClearSearch() {
	tv.searchTerm = ""
	tv.searchHits = nil
}

// Render renders the visible portion of the transcript.
func (tv *TranscriptView) Render() string {
	if len(tv.entries) == 0 {
		return "  (empty transcript)"
	}

	end := tv.offset + tv.height
	if end > len(tv.entries) {
		end = len(tv.entries)
	}
	start := tv.offset
	if start < 0 {
		start = 0
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		e := tv.entries[i]
		line := formatTranscriptEntry(e, tv.width, tv.isSearchHit(i))
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Scroll info
	if len(tv.entries) > tv.height {
		pct := 0
		maxOff := len(tv.entries) - tv.height
		if maxOff > 0 {
			pct = tv.offset * 100 / maxOff
		}
		sb.WriteString(fmt.Sprintf("  ─── %d%% (%d/%d entries) ───",
			pct, tv.offset+tv.height, len(tv.entries)))
	}

	return sb.String()
}

// EntryCount returns total entries.
func (tv *TranscriptView) EntryCount() int {
	return len(tv.entries)
}

func (tv *TranscriptView) isSearchHit(idx int) bool {
	for _, h := range tv.searchHits {
		if h == idx {
			return true
		}
	}
	return false
}

func formatTranscriptEntry(e TranscriptEntry, width int, highlight bool) string {
	ts := e.Timestamp.Format("15:04:05")
	prefix := ""

	switch e.Role {
	case "user":
		prefix = "❯ You"
	case "assistant":
		prefix = "⏺ Assistant"
	case "system":
		prefix = "▶ System"
	case "tool_use":
		if e.ToolName != "" {
			prefix = "⚙ " + e.ToolName
		} else {
			prefix = "⚙ Tool"
		}
	case "tool_result":
		prefix = "  ⎿ Result"
		if e.IsError {
			prefix = "  ✗ Error"
		}
	case "error":
		prefix = "⚠ Error"
	default:
		prefix = "  " + e.Role
	}

	content := e.Content
	maxContent := width - len(ts) - len(prefix) - 6
	if maxContent < 20 {
		maxContent = 20
	}
	if len(content) > maxContent {
		content = content[:maxContent] + "…"
	}
	// Replace newlines
	content = strings.ReplaceAll(content, "\n", " ↵ ")

	line := fmt.Sprintf("  %s  %s  %s", ts, prefix, content)

	if highlight {
		line = "▸" + line[1:]
	}

	return line
}
