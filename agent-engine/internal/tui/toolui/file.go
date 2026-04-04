package toolui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ReadToolUI renders file read tool use.
// Layout matches claude-code-main's FileReadTool:
//
//	● Read (/src/main.go)
//	  ⎿  Read 42 lines (12ms)
type ReadToolUI struct {
	theme ToolUITheme
}

// NewReadToolUI creates a read tool renderer.
func NewReadToolUI(theme ToolUITheme) *ReadToolUI {
	return &ReadToolUI{theme: theme}
}

// RenderStart renders a read tool header line:
//
//	● Read (/path/to/file)
func (r *ReadToolUI) RenderStart(dotView, filePath string, lineRange string, verbose bool) string {
	displayPath := filePath
	if !verbose {
		displayPath = shortenPath(filePath)
	}
	params := displayPath
	if lineRange != "" {
		params += " " + lineRange
	}
	return RenderToolHeader(dotView, "Read", params, r.theme)
}

// RenderResult renders a read tool result with ⎿ connector:
//
//	⎿  Read 42 lines (12ms)
func (r *ReadToolUI) RenderResult(content string, lineCount int, elapsed time.Duration, width int, verbose bool) string {
	var sb strings.Builder

	msg := fmt.Sprintf("Read %d lines (%s)", lineCount, elapsed.Truncate(time.Millisecond))
	sb.WriteString(RenderResponseLine(r.theme.Dim.Render(msg), r.theme))

	// Show content preview in verbose mode
	if verbose && content != "" {
		lines := strings.Split(content, "\n")
		maxPreview := 5
		if len(lines) > maxPreview {
			sb.WriteString("\n")
			for _, line := range lines[:maxPreview] {
				sb.WriteString(r.theme.TreeConn.Render("  │ "))
				sb.WriteString(r.theme.Output.Render(truncateLine(line, width-6)))
				sb.WriteString("\n")
			}
			sb.WriteString(r.theme.Dim.Render(fmt.Sprintf("  │ … (%d more lines)", len(lines)-maxPreview)))
		} else {
			sb.WriteString("\n")
			for _, line := range lines {
				sb.WriteString(r.theme.TreeConn.Render("  │ "))
				sb.WriteString(r.theme.Output.Render(truncateLine(line, width-6)))
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// GlobToolUI renders glob/search tool use.
// Layout matches claude-code-main's GlobTool:
//
//	● Search (pattern: "*.go", path: "/src")
//	  ⎿  Found 12 files
type GlobToolUI struct {
	theme ToolUITheme
}

// NewGlobToolUI creates a glob tool renderer.
func NewGlobToolUI(theme ToolUITheme) *GlobToolUI {
	return &GlobToolUI{theme: theme}
}

// RenderStart renders a glob tool header line:
//
//	● Search (pattern: "*.go", path: "/src")
func (g *GlobToolUI) RenderStart(dotView, pattern, directory string, verbose bool) string {
	params := fmt.Sprintf("pattern: %q", pattern)
	if directory != "" {
		dir := directory
		if !verbose {
			dir = shortenDir(dir)
		}
		params += fmt.Sprintf(", path: %q", dir)
	}
	return RenderToolHeader(dotView, "Search", params, g.theme)
}

// RenderResult renders glob results with ⎿ connector:
//
//	⎿  Found 12 files  (Ctrl+O to expand)
func (g *GlobToolUI) RenderResult(files []string, elapsed time.Duration, verbose bool) string {
	var sb strings.Builder

	count := len(files)
	label := "files"
	if count == 1 {
		label = "file"
	}
	summary := fmt.Sprintf("Found %d %s", count, label)

	if !verbose && count > 0 {
		summary += "  " + g.theme.Dim.Render("(Ctrl+O to expand)")
	}

	sb.WriteString(RenderResponseLine(g.theme.Dim.Render(summary), g.theme))

	// Show file list in verbose mode
	if verbose && len(files) > 0 {
		maxShow := 8
		show := files
		if len(show) > maxShow {
			show = show[:maxShow]
		}
		for _, f := range show {
			sb.WriteString("\n")
			sb.WriteString(g.theme.TreeConn.Render("  │ "))
			sb.WriteString(g.theme.FilePath.Render(filepath.Base(f)))
		}
		if len(files) > maxShow {
			sb.WriteString("\n")
			sb.WriteString(g.theme.Dim.Render(fmt.Sprintf("  │ … (%d more)", len(files)-maxShow)))
		}
	}

	return sb.String()
}

// GrepToolUI renders grep/search tool use.
// Layout matches claude-code-main's GrepTool:
//
//	● Grep (pattern: "TODO", path: "/src")
//	  ⎿  Found 12 results across 5 files  (Ctrl+O to expand)
type GrepToolUI struct {
	theme ToolUITheme
}

// NewGrepToolUI creates a grep tool renderer.
func NewGrepToolUI(theme ToolUITheme) *GrepToolUI {
	return &GrepToolUI{theme: theme}
}

// RenderStart renders a grep tool header line:
//
//	● Grep (pattern: "TODO", path: "/src")
func (g *GrepToolUI) RenderStart(dotView, pattern, directory string, verbose bool) string {
	params := fmt.Sprintf("pattern: %q", pattern)
	if directory != "" {
		dir := directory
		if !verbose {
			dir = shortenDir(dir)
		}
		params += fmt.Sprintf(", path: %q", dir)
	}
	return RenderToolHeader(dotView, "Grep", params, g.theme)
}

// RenderResult renders grep results with ⎿ connector:
//
//	⎿  Found 12 results across 5 files  (Ctrl+O to expand)
func (g *GrepToolUI) RenderResult(matchCount, fileCount int, output string, elapsed time.Duration, width int, verbose bool) string {
	var sb strings.Builder

	// Build summary like claude-code: "Found N results across M files"
	resultLabel := "results"
	if matchCount == 1 {
		resultLabel = "result"
	}
	fileLabel := "files"
	if fileCount == 1 {
		fileLabel = "file"
	}
	summary := fmt.Sprintf("Found %d %s across %d %s", matchCount, resultLabel, fileCount, fileLabel)

	if !verbose && matchCount > 0 {
		summary += "  " + g.theme.Dim.Render("(Ctrl+O to expand)")
	}

	sb.WriteString(RenderResponseLine(g.theme.Dim.Render(summary), g.theme))

	// Show output in verbose mode
	if verbose && output != "" {
		lines := strings.Split(output, "\n")
		maxShow := 8
		show := lines
		if len(show) > maxShow {
			show = show[:maxShow]
		}
		for _, line := range show {
			sb.WriteString("\n")
			sb.WriteString(g.theme.TreeConn.Render("  │ "))
			sb.WriteString(g.theme.Output.Render(truncateLine(line, width-6)))
		}
		if len(lines) > maxShow {
			sb.WriteString("\n")
			sb.WriteString(g.theme.Dim.Render(fmt.Sprintf("  │ … (%d more lines)", len(lines)-maxShow)))
		}
	}

	return sb.String()
}

// WebToolUI renders web fetch/search tool use.
// Layout matches claude-code-main's WebFetchTool/WebSearchTool.
type WebToolUI struct {
	theme ToolUITheme
}

// NewWebToolUI creates a web tool renderer.
func NewWebToolUI(theme ToolUITheme) *WebToolUI {
	return &WebToolUI{theme: theme}
}

// RenderStart renders a web tool header line:
//
//	● WebSearch (query text)
func (w *WebToolUI) RenderStart(dotView, toolName, query string) string {
	return RenderToolHeader(dotView, toolName, query, w.theme)
}

// RenderResult renders a web tool result with ⎿ connector.
func (w *WebToolUI) RenderResult(content string, elapsed time.Duration, width int) string {
	var sb strings.Builder

	msg := fmt.Sprintf("Done (%s)", elapsed.Truncate(time.Millisecond))
	sb.WriteString(RenderResponseLine(w.theme.Dim.Render(msg), w.theme))

	if content != "" {
		lines := strings.Split(content, "\n")
		maxShow := 5
		show := lines
		if len(show) > maxShow {
			show = show[:maxShow]
		}
		for _, line := range show {
			sb.WriteString("\n")
			sb.WriteString(w.theme.TreeConn.Render("  │ "))
			sb.WriteString(w.theme.Output.Render(truncateLine(line, width-6)))
		}
		if len(lines) > maxShow {
			sb.WriteString("\n")
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
