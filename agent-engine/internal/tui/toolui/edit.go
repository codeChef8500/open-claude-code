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
