package toolui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ReadToolUI renders file read tool use.
type ReadToolUI struct {
	theme ToolUITheme
}

// NewReadToolUI creates a read tool renderer.
func NewReadToolUI(theme ToolUITheme) *ReadToolUI {
	return &ReadToolUI{theme: theme}
}

// RenderStart renders a read tool invocation.
func (r *ReadToolUI) RenderStart(filePath string, lineRange string) string {
	var sb strings.Builder
	sb.WriteString(r.theme.ToolIcon.Render("⚙ Read"))
	sb.WriteString("\n")
	sb.WriteString(r.theme.TreeConn.Render("  ⎿ "))
	sb.WriteString(r.theme.FilePath.Render(filePath))
	if lineRange != "" {
		sb.WriteString(r.theme.Dim.Render(" " + lineRange))
	}
	return sb.String()
}

// RenderResult renders a read tool result with content preview.
func (r *ReadToolUI) RenderResult(content string, lineCount int, elapsed time.Duration, width int) string {
	var sb strings.Builder
	sb.WriteString(r.theme.Success.Render(fmt.Sprintf("  ✓ Read %d lines (%s)", lineCount, elapsed.Truncate(time.Millisecond))))

	// Show first few lines as preview
	if content != "" {
		lines := strings.Split(content, "\n")
		maxPreview := 5
		if len(lines) > maxPreview {
			sb.WriteString("\n")
			for _, line := range lines[:maxPreview] {
				sb.WriteString(r.theme.Output.Render("  │ " + truncateLine(line, width-6)))
				sb.WriteString("\n")
			}
			sb.WriteString(r.theme.Dim.Render(fmt.Sprintf("  │ … (%d more lines)", len(lines)-maxPreview)))
		} else {
			sb.WriteString("\n")
			for _, line := range lines {
				sb.WriteString(r.theme.Output.Render("  │ " + truncateLine(line, width-6)))
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// GlobToolUI renders glob/search tool use.
type GlobToolUI struct {
	theme ToolUITheme
}

// NewGlobToolUI creates a glob tool renderer.
func NewGlobToolUI(theme ToolUITheme) *GlobToolUI {
	return &GlobToolUI{theme: theme}
}

// RenderStart renders a glob tool invocation.
func (g *GlobToolUI) RenderStart(pattern, directory string) string {
	var sb strings.Builder
	sb.WriteString(g.theme.ToolIcon.Render("⚙ Glob"))
	sb.WriteString("\n")
	sb.WriteString(g.theme.TreeConn.Render("  ⎿ "))
	sb.WriteString(g.theme.Code.Render(pattern))
	if directory != "" {
		sb.WriteString(g.theme.Dim.Render(" in " + shortenDir(directory)))
	}
	return sb.String()
}

// RenderResult renders glob results.
func (g *GlobToolUI) RenderResult(files []string, elapsed time.Duration) string {
	var sb strings.Builder
	sb.WriteString(g.theme.Success.Render(fmt.Sprintf("  ✓ Found %d files (%s)", len(files), elapsed.Truncate(time.Millisecond))))

	maxShow := 8
	if len(files) > 0 {
		sb.WriteString("\n")
		show := files
		if len(show) > maxShow {
			show = show[:maxShow]
		}
		for _, f := range show {
			sb.WriteString(g.theme.Dim.Render("  │ "))
			sb.WriteString(g.theme.FilePath.Render(filepath.Base(f)))
			sb.WriteString("\n")
		}
		if len(files) > maxShow {
			sb.WriteString(g.theme.Dim.Render(fmt.Sprintf("  │ … (%d more)", len(files)-maxShow)))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// GrepToolUI renders grep/search tool use.
type GrepToolUI struct {
	theme ToolUITheme
}

// NewGrepToolUI creates a grep tool renderer.
func NewGrepToolUI(theme ToolUITheme) *GrepToolUI {
	return &GrepToolUI{theme: theme}
}

// RenderStart renders a grep tool invocation.
func (g *GrepToolUI) RenderStart(pattern, directory string) string {
	var sb strings.Builder
	sb.WriteString(g.theme.ToolIcon.Render("⚙ Grep"))
	sb.WriteString("\n")
	sb.WriteString(g.theme.TreeConn.Render("  ⎿ "))
	sb.WriteString(g.theme.Code.Render(fmt.Sprintf("%q", pattern)))
	if directory != "" {
		sb.WriteString(g.theme.Dim.Render(" in " + shortenDir(directory)))
	}
	return sb.String()
}

// RenderResult renders grep results.
func (g *GrepToolUI) RenderResult(matchCount int, output string, elapsed time.Duration, width int) string {
	var sb strings.Builder
	sb.WriteString(g.theme.Success.Render(fmt.Sprintf("  ✓ %d matches (%s)", matchCount, elapsed.Truncate(time.Millisecond))))

	if output != "" {
		lines := strings.Split(output, "\n")
		maxShow := 8
		sb.WriteString("\n")
		show := lines
		if len(show) > maxShow {
			show = show[:maxShow]
		}
		for _, line := range show {
			sb.WriteString(g.theme.Output.Render("  │ " + truncateLine(line, width-6)))
			sb.WriteString("\n")
		}
		if len(lines) > maxShow {
			sb.WriteString(g.theme.Dim.Render(fmt.Sprintf("  │ … (%d more lines)", len(lines)-maxShow)))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// WebToolUI renders web fetch/search tool use.
type WebToolUI struct {
	theme ToolUITheme
}

// NewWebToolUI creates a web tool renderer.
func NewWebToolUI(theme ToolUITheme) *WebToolUI {
	return &WebToolUI{theme: theme}
}

// RenderStart renders a web tool invocation.
func (w *WebToolUI) RenderStart(toolName, query string) string {
	var sb strings.Builder
	sb.WriteString(w.theme.ToolIcon.Render("⚙ " + toolName))
	sb.WriteString("\n")
	sb.WriteString(w.theme.TreeConn.Render("  ⎿ "))
	sb.WriteString(w.theme.Code.Render(query))
	return sb.String()
}

// RenderResult renders a web tool result.
func (w *WebToolUI) RenderResult(content string, elapsed time.Duration, width int) string {
	var sb strings.Builder
	sb.WriteString(w.theme.Success.Render(fmt.Sprintf("  ✓ Done (%s)", elapsed.Truncate(time.Millisecond))))

	if content != "" {
		lines := strings.Split(content, "\n")
		maxShow := 5
		sb.WriteString("\n")
		show := lines
		if len(show) > maxShow {
			show = show[:maxShow]
		}
		for _, line := range show {
			sb.WriteString(w.theme.Output.Render("  │ " + truncateLine(line, width-6)))
			sb.WriteString("\n")
		}
		if len(lines) > maxShow {
			sb.WriteString(w.theme.Dim.Render(fmt.Sprintf("  │ … (%d more lines)", len(lines)-maxShow)))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// shortenDir shortens a directory path for display.
func shortenDir(dir string) string {
	if len(dir) <= 40 {
		return dir
	}
	return "…" + dir[len(dir)-39:]
}
