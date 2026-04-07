package toolui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// EditToolUI renders file edit tool use with diff display.
// Layout matches claude-code-main's FileEditTool:
//
//	● Update (/src/main.go)
//	  ⎿  Applied (23ms) — 5 lines changed
//	     - old line
//	     + new line
type EditToolUI struct {
	theme ToolUITheme
}

// NewEditToolUI creates an edit tool renderer.
func NewEditToolUI(theme ToolUITheme) *EditToolUI {
	return &EditToolUI{theme: theme}
}

// RenderStart renders an edit tool header line:
//
//	● Update (/src/main.go)
//
// toolName should be "Update" or "Create" depending on whether old_string is empty.
func (e *EditToolUI) RenderStart(dotView, toolName, filePath string, verbose bool) string {
	displayPath := filePath
	if !verbose {
		displayPath = shortenPath(filePath)
	}
	return RenderToolHeader(dotView, toolName, displayPath, e.theme)
}

// RenderResult renders the edit result with ⎿ connector:
//
//	⎿  Applied (23ms) — 5 lines changed
//	   - old line
//	   + new line
func (e *EditToolUI) RenderResult(success bool, elapsed time.Duration, linesChanged int, oldText, newText string, width int) string {
	var sb strings.Builder

	if success {
		msg := fmt.Sprintf("Applied (%s)", elapsed.Truncate(time.Millisecond))
		if linesChanged > 0 {
			msg += fmt.Sprintf(" — %d lines changed", linesChanged)
		}
		sb.WriteString(RenderResponseLine(e.theme.Dim.Render(msg), e.theme))
	} else {
		sb.WriteString(RenderResponseLine(e.theme.Error.Render("Edit failed"), e.theme))
	}

	// Show diff if available
	if oldText != "" || newText != "" {
		diff := e.renderDiff(oldText, newText, width)
		if diff != "" {
			sb.WriteString("\n")
			sb.WriteString(diff)
		}
	}

	return sb.String()
}

// RenderResultSimple renders a simple result without diff.
func (e *EditToolUI) RenderResultSimple(success bool, elapsed time.Duration, linesChanged int) string {
	if success {
		msg := fmt.Sprintf("Applied (%s)", elapsed.Truncate(time.Millisecond))
		if linesChanged > 0 {
			msg += fmt.Sprintf(" — %d lines changed", linesChanged)
		}
		return RenderResponseLine(e.theme.Dim.Render(msg), e.theme)
	}
	return RenderResponseLine(e.theme.Error.Render("Edit failed"), e.theme)
}

// RenderResultReplaceAll renders an edit result for replace_all with count.
func (e *EditToolUI) RenderResultReplaceAll(elapsed time.Duration, replacements int) string {
	msg := fmt.Sprintf("Applied (%s) — replaced %d occurrences", elapsed.Truncate(time.Millisecond), replacements)
	return RenderResponseLine(e.theme.Dim.Render(msg), e.theme)
}

// EditHunk is the minimal data for rendering a structured patch hunk.
type EditHunk struct {
	OldStart int
	NewStart int
	Lines    []string // prefixed with " ", "+", or "-"
}

// RenderStructuredPatch renders structured patch hunks matching claude-code-main's
// FileEditToolUpdatedMessage diff display.
func (e *EditToolUI) RenderStructuredPatch(hunks []EditHunk, width int) string {
	if len(hunks) == 0 {
		return ""
	}
	var sb strings.Builder
	maxHunks := 3
	maxLinesPerHunk := 10

	for hi, h := range hunks {
		if hi >= maxHunks {
			sb.WriteString("\n")
			sb.WriteString(e.theme.Dim.Render(fmt.Sprintf("     … and %d more hunks", len(hunks)-maxHunks)))
			break
		}
		if hi > 0 {
			sb.WriteString("\n")
		}
		shown := 0
		for _, line := range h.Lines {
			if shown >= maxLinesPerHunk {
				sb.WriteString("\n")
				sb.WriteString(e.theme.Dim.Render(fmt.Sprintf("     … (%d more lines in hunk)", len(h.Lines)-maxLinesPerHunk)))
				break
			}
			if shown > 0 {
				sb.WriteString("\n")
			}
			if len(line) == 0 {
				sb.WriteString("     ")
				shown++
				continue
			}
			prefix := line[0]
			content := line[1:]
			switch prefix {
			case '+':
				sb.WriteString(e.theme.DiffAdd.Render("     + "))
				sb.WriteString(e.theme.DiffAdd.Render(truncateLine(content, width-8)))
			case '-':
				sb.WriteString(e.theme.DiffDel.Render("     - "))
				sb.WriteString(e.theme.DiffDel.Render(truncateLine(content, width-8)))
			default:
				sb.WriteString("       ")
				sb.WriteString(e.theme.Dim.Render(truncateLine(content, width-8)))
			}
			shown++
		}
	}
	return sb.String()
}

// RenderRejected renders a rejected (permission-denied) edit.
func (e *EditToolUI) RenderRejected(filePath string, isNewFile bool) string {
	var msg string
	if isNewFile {
		msg = "File creation rejected"
	} else {
		msg = "Edit rejected"
	}
	return RenderResponseLine(e.theme.Error.Render(msg), e.theme)
}

// renderDiff creates a word-level diff view matching claude-code-main's
// FileEditTool highlighting. Changed words get background color.
func (e *EditToolUI) renderDiff(oldText, newText string, width int) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldText, newText, true)
	diffs = dmp.DiffCleanupSemantic(diffs)

	// Build removed and added line buffers with word-level highlighting.
	var delBuf, addBuf strings.Builder
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			delBuf.WriteString(e.theme.DiffDelWord.Render(d.Text))
		case diffmatchpatch.DiffInsert:
			addBuf.WriteString(e.theme.DiffAddWord.Render(d.Text))
		case diffmatchpatch.DiffEqual:
			delBuf.WriteString(e.theme.DiffDel.Render(d.Text))
			addBuf.WriteString(e.theme.DiffAdd.Render(d.Text))
		}
	}

	// Split into lines and render with prefix
	maxLines := 12
	var sb strings.Builder
	renderDiffLines := func(prefix string, raw string) {
		lines := strings.Split(raw, "\n")
		shown := 0
		for _, line := range lines {
			if shown >= maxLines/2 {
				if len(lines) > maxLines/2 {
					sb.WriteString(e.theme.Dim.Render(fmt.Sprintf("     … (%d more lines)", len(lines)-maxLines/2)))
					sb.WriteString("\n")
				}
				break
			}
			sb.WriteString(prefix)
			sb.WriteString(truncateLine(line, width-8))
			sb.WriteString("\n")
			shown++
		}
	}

	delStr := delBuf.String()
	addStr := addBuf.String()

	if delStr != "" {
		renderDiffLines(e.theme.DiffDel.Render("     - "), delStr)
	}
	if addStr != "" {
		renderDiffLines(e.theme.DiffAdd.Render("     + "), addStr)
	}

	return strings.TrimRight(sb.String(), "\n")
}

// WriteToolUI renders file write tool use.
// Layout matches claude-code-main's FileWriteTool:
//
//	● Write (/path/to/file)
//	  ⎿  Written (12ms)
type WriteToolUI struct {
	theme ToolUITheme
}

// NewWriteToolUI creates a write tool renderer.
func NewWriteToolUI(theme ToolUITheme) *WriteToolUI {
	return &WriteToolUI{theme: theme}
}

// RenderStart renders a write tool header line:
//
//	● Write (/path/to/file)
func (w *WriteToolUI) RenderStart(dotView, filePath string, verbose bool) string {
	displayPath := filePath
	if !verbose {
		displayPath = shortenPath(filePath)
	}
	return RenderToolHeader(dotView, "Write", displayPath, w.theme)
}

// RenderResult renders the write result with ⎿ connector.
func (w *WriteToolUI) RenderResult(success bool, elapsed time.Duration) string {
	if success {
		msg := fmt.Sprintf("Written (%s)", elapsed.Truncate(time.Millisecond))
		return RenderResponseLine(w.theme.Dim.Render(msg), w.theme)
	}
	return RenderResponseLine(w.theme.Error.Render("Write failed"), w.theme)
}

// shortenPath shortens a file path for compact display.
func shortenPath(path string) string {
	if len(path) <= 50 {
		return path
	}
	return "…" + path[len(path)-49:]
}
