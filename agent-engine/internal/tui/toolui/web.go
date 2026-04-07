package toolui

import (
	"fmt"
	"strings"
	"time"
)

// ─── WebSearchToolUI ────────────────────────────────────────────────────────

// WebSearchToolUI renders web search tool use.
// Layout matches claude-code-main's WebSearchTool.
//
//	● Search (golang concurrency patterns)
//	  ⎿  Found 5 results (1.2s)
//	     - [Title 1](url1)
//	     - [Title 2](url2)
type WebSearchToolUI struct {
	theme ToolUITheme
}

// NewWebSearchToolUI creates a web search tool renderer.
func NewWebSearchToolUI(theme ToolUITheme) *WebSearchToolUI {
	return &WebSearchToolUI{theme: theme}
}

// RenderStart renders a web search tool header line:
//
//	● Search (query text)
func (w *WebSearchToolUI) RenderStart(dotView, query string) string {
	return RenderToolHeader(dotView, "Search", query, w.theme)
}

// RenderProgress renders a search progress line:
//
//	⎿  Searching: "golang concurrency"…
func (w *WebSearchToolUI) RenderProgress(query string) string {
	q := query
	if len(q) > 60 {
		q = q[:60] + "…"
	}
	return RenderResponseLine(w.theme.Dim.Render(fmt.Sprintf(`Searching: "%s"…`, q)), w.theme)
}

// RenderResult renders the search result summary:
//
//	⎿  Found 5 results (1.2s)
//	   - [Title 1](url1)
//	   - [Title 2](url2)
func (w *WebSearchToolUI) RenderResult(numResults int, elapsed time.Duration, hits []SearchHitDisplay, width int) string {
	var sb strings.Builder

	msg := fmt.Sprintf("Found %d results (%s)", numResults, formatDuration(elapsed))
	sb.WriteString(RenderResponseLine(w.theme.Dim.Render(msg), w.theme))

	maxShow := 8
	for i, hit := range hits {
		if i >= maxShow {
			sb.WriteString("\n")
			sb.WriteString(w.theme.TreeConn.Render("  │ "))
			sb.WriteString(w.theme.Dim.Render(fmt.Sprintf("… and %d more results", len(hits)-maxShow)))
			break
		}
		sb.WriteString("\n")
		sb.WriteString(w.theme.TreeConn.Render("  │ "))
		line := fmt.Sprintf("- [%s](%s)", hit.Title, hit.URL)
		sb.WriteString(w.theme.Output.Render(truncateLine(line, width-6)))
	}

	return sb.String()
}

// GetToolUseSummary returns the query for summary display.
func (w *WebSearchToolUI) GetToolUseSummary(query string) string {
	if len(query) > 80 {
		return query[:80] + "…"
	}
	return query
}

// SearchHitDisplay is the minimal data needed to render a search hit.
type SearchHitDisplay struct {
	Title string
	URL   string
}

// ─── WebFetchToolUI ─────────────────────────────────────────────────────────

// WebFetchToolUI renders web fetch tool use.
// Layout matches claude-code-main's WebFetchTool.
//
//	● Fetch (https://example.com/page)
//	  ⎿  Received 2.3KB (200 OK) in 1.5s
type WebFetchToolUI struct {
	theme ToolUITheme
}

// NewWebFetchToolUI creates a web fetch tool renderer.
func NewWebFetchToolUI(theme ToolUITheme) *WebFetchToolUI {
	return &WebFetchToolUI{theme: theme}
}

// RenderStart renders a web fetch tool header line:
//
//	● Fetch (https://example.com/page)
func (w *WebFetchToolUI) RenderStart(dotView, urlStr string) string {
	displayURL := urlStr
	if len(displayURL) > 80 {
		displayURL = displayURL[:80] + "…"
	}
	return RenderToolHeader(dotView, "Fetch", displayURL, w.theme)
}

// RenderProgress renders a fetch progress line:
//
//	⎿  Fetching…
func (w *WebFetchToolUI) RenderProgress() string {
	return RenderResponseLine(w.theme.Dim.Render("Fetching…"), w.theme)
}

// RenderProgressPhase renders a fetch progress line with phase details:
//
//	⎿  Downloading… 2.3KB (200)
func (w *WebFetchToolUI) RenderProgressPhase(phase string, bytesRead, statusCode int) string {
	var parts []string
	switch phase {
	case "connecting":
		parts = append(parts, "Connecting…")
	case "downloading":
		parts = append(parts, "Downloading…")
		if bytesRead > 0 {
			parts = append(parts, formatBytes(bytesRead))
		}
	case "processing":
		parts = append(parts, "Processing…")
		if bytesRead > 0 {
			parts = append(parts, formatBytes(bytesRead))
		}
	default:
		parts = append(parts, "Fetching…")
	}
	if statusCode > 0 {
		parts = append(parts, fmt.Sprintf("(%d)", statusCode))
	}
	return RenderResponseLine(w.theme.Dim.Render(strings.Join(parts, " ")), w.theme)
}

// RenderResult renders the fetch result with size and status:
//
//	⎿  Received 2.3KB (200 OK) in 1.5s
func (w *WebFetchToolUI) RenderResult(sizeBytes int, statusCode int, statusText string, elapsed time.Duration) string {
	sizeStr := formatBytes(sizeBytes)
	var msg string
	if statusCode >= 400 {
		msg = w.theme.Error.Render(fmt.Sprintf("Error: %d %s (%s)", statusCode, statusText, formatDuration(elapsed)))
	} else {
		msg = fmt.Sprintf("Received %s (%d %s) in %s",
			w.theme.ToolIcon.Bold(true).Render(sizeStr),
			statusCode, statusText, formatDuration(elapsed))
	}
	return RenderResponseLine(msg, w.theme)
}

// RenderRedirect renders a redirect notification:
//
//	⎿  Redirected → https://other.example.com/page
func (w *WebFetchToolUI) RenderRedirect(redirectURL string) string {
	return RenderResponseLine(w.theme.Dim.Render("Redirected → "+redirectURL), w.theme)
}

// GetToolUseSummary returns the truncated URL.
func (w *WebFetchToolUI) GetToolUseSummary(urlStr string) string {
	if len(urlStr) > 80 {
		return urlStr[:80] + "…"
	}
	return urlStr
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// formatBytes formats a byte count for display (e.g. "2.3KB", "1.5MB").
func formatBytes(b int) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// formatDuration formats a duration for display (e.g. "1.2s", "340ms").
func formatDuration(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
