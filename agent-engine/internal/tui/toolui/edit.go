package toolui

import (
	"fmt"
	"strings"
	"time"
)

// EditToolUI renders file edit tool use with diff display.
type EditToolUI struct {
	theme ToolUITheme
}

// NewEditToolUI creates an edit tool renderer.
func NewEditToolUI(theme ToolUITheme) *EditToolUI {
	return &EditToolUI{theme: theme}
}

// RenderStart renders an edit tool invocation.
func (e *EditToolUI) RenderStart(filePath, oldText, newText string, width int) string {
	var sb strings.Builder

	sb.WriteString(e.theme.ToolIcon.Render("⚙ Edit"))
	sb.WriteString("\n")
	sb.WriteString(e.theme.TreeConn.Render("  ⎿ "))
	sb.WriteString(e.theme.FilePath.Render(filePath))

	if oldText != "" && newText != "" {
		sb.WriteString("\n")
		sb.WriteString(e.renderDiff(oldText, newText, width))
	}

	return sb.String()
}

// RenderResult renders the edit result.
func (e *EditToolUI) RenderResult(success bool, elapsed time.Duration, linesChanged int) string {
	if success {
		msg := fmt.Sprintf("  ✓ Applied (%s)", elapsed.Truncate(time.Millisecond))
		if linesChanged > 0 {
			msg += fmt.Sprintf(" — %d lines changed", linesChanged)
		}
		return e.theme.Success.Render(msg)
	}
	return e.theme.Error.Render("  ✗ Edit failed")
}

// renderDiff creates a simplified diff view.
func (e *EditToolUI) renderDiff(oldText, newText string, width int) string {
	var sb strings.Builder
	maxDiffLines := 10

	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	// Simple line-by-line diff rendering
	delCount := 0
	addCount := 0

	// Show removed lines
	for i, line := range oldLines {
		if i >= maxDiffLines/2 {
			if len(oldLines) > maxDiffLines/2 {
				sb.WriteString(e.theme.Dim.Render(fmt.Sprintf("    … (%d more removed)", len(oldLines)-maxDiffLines/2)))
				sb.WriteString("\n")
			}
			break
		}
		sb.WriteString(e.theme.DiffDel.Render("  - " + truncateLine(line, width-6)))
		sb.WriteString("\n")
		delCount++
	}

	// Show added lines
	for i, line := range newLines {
		if i >= maxDiffLines/2 {
			if len(newLines) > maxDiffLines/2 {
				sb.WriteString(e.theme.Dim.Render(fmt.Sprintf("    … (%d more added)", len(newLines)-maxDiffLines/2)))
				sb.WriteString("\n")
			}
			break
		}
		sb.WriteString(e.theme.DiffAdd.Render("  + " + truncateLine(line, width-6)))
		sb.WriteString("\n")
		addCount++
	}

	_ = delCount
	_ = addCount

	return strings.TrimRight(sb.String(), "\n")
}

// WriteToolUI renders file write tool use.
type WriteToolUI struct {
	theme ToolUITheme
}

// NewWriteToolUI creates a write tool renderer.
func NewWriteToolUI(theme ToolUITheme) *WriteToolUI {
	return &WriteToolUI{theme: theme}
}

// RenderStart renders a write tool invocation.
func (w *WriteToolUI) RenderStart(filePath string, lineCount int) string {
	var sb strings.Builder
	sb.WriteString(w.theme.ToolIcon.Render("⚙ Write"))
	sb.WriteString("\n")
	sb.WriteString(w.theme.TreeConn.Render("  ⎿ "))
	sb.WriteString(w.theme.FilePath.Render(filePath))
	if lineCount > 0 {
		sb.WriteString(w.theme.Dim.Render(fmt.Sprintf(" (%d lines)", lineCount)))
	}
	return sb.String()
}

// RenderResult renders the write result.
func (w *WriteToolUI) RenderResult(success bool, elapsed time.Duration) string {
	if success {
		return w.theme.Success.Render(fmt.Sprintf("  ✓ Written (%s)", elapsed.Truncate(time.Millisecond)))
	}
	return w.theme.Error.Render("  ✗ Write failed")
}
