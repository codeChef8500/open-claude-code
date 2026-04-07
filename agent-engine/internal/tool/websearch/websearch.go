package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

const (
	defaultMaxResults = 10
	maxResultsCap     = 50
	httpTimeout       = 15 * time.Second
)

type Input struct {
	Query          string   `json:"query"`
	MaxResults     int      `json:"max_results,omitempty"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
}

// SearchHit represents a single search result.
type SearchHit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

// Output is the structured output of a WebSearch call.
type Output struct {
	Query           string      `json:"query"`
	Results         []SearchHit `json:"results"`
	DurationSeconds float64     `json:"durationSeconds"`
}

// WebSearchTool uses a configurable search backend (default: DuckDuckGo JSON API).
type WebSearchTool struct {
	tool.BaseTool
	apiKey   string
	baseURL  string
	client   *http.Client
	provider SearchProvider // pluggable search backend
}

func New(apiKey, baseURL string) *WebSearchTool {
	if baseURL == "" {
		baseURL = "https://api.duckduckgo.com"
	}
	t := &WebSearchTool{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: httpTimeout},
	}
	// Default provider is the built-in DuckDuckGo backend.
	t.provider = &ddgProvider{tool: t}
	return t
}

// SetProvider replaces the search backend (e.g. SerpAPI, Brave).
func (t *WebSearchTool) SetProvider(p SearchProvider) { t.provider = p }

func (t *WebSearchTool) Name() string                             { return "WebSearch" }
func (t *WebSearchTool) UserFacingName() string                   { return "Search" }
func (t *WebSearchTool) Description() string                      { return "Search the web for information." }
func (t *WebSearchTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *WebSearchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *WebSearchTool) MaxResultSizeChars() int                  { return 100_000 }
func (t *WebSearchTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *WebSearchTool) IsSearchOrRead(_ json.RawMessage) engine.SearchOrReadInfo {
	return engine.SearchOrReadInfo{IsSearch: true}
}

func (t *WebSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"query":{"type":"string","description":"The search query to use."},
			"max_results":{"type":"integer","description":"Maximum number of results (default 10, max 50)."},
			"allowed_domains":{"type":"array","items":{"type":"string"},"description":"Only include search results from these domains."},
			"blocked_domains":{"type":"array","items":{"type":"string"},"description":"Never include search results from these domains."}
		},
		"required":["query"]
	}`)
}

func (t *WebSearchTool) OutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"query":{"type":"string","description":"The search query that was executed."},
			"results":{"type":"array","items":{"type":"object","properties":{"title":{"type":"string"},"url":{"type":"string"},"snippet":{"type":"string"}}},"description":"Search results."},
			"durationSeconds":{"type":"number","description":"Time taken to complete the search."}
		}
	}`)
}

func (t *WebSearchTool) GetToolUseSummary(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil || in.Query == "" {
		return ""
	}
	q := in.Query
	if len(q) > 80 {
		q = q[:80] + "\u2026"
	}
	return q
}

func (t *WebSearchTool) GetActivityDescription(input json.RawMessage) string {
	s := t.GetToolUseSummary(input)
	if s != "" {
		return "Searching: " + s
	}
	return "Searching the web"
}

func (t *WebSearchTool) ToAutoClassifierInput(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.Query
}

func (t *WebSearchTool) Prompt(_ *tool.UseContext) string {
	currentMonthYear := time.Now().Format("January 2006")
	return fmt.Sprintf(`- Allows the agent to search the web and use the results to inform responses
- Provides up-to-date information for current events and recent data
- Returns search result information formatted as search result blocks, including links as markdown hyperlinks
- Use this tool for accessing information beyond the agent's knowledge cutoff
- Searches are performed automatically within a single API call

CRITICAL REQUIREMENT - You MUST follow this:
  - After answering the user's question, you MUST include a "Sources:" section at the end of your response
  - In the Sources section, list all relevant URLs from the search results as markdown hyperlinks: [Title](URL)
  - This is MANDATORY - never skip including sources in your response
  - Example format:

    [Your answer here]

    Sources:
    - [Source Title 1](https://example.com/1)
    - [Source Title 2](https://example.com/2)

Usage notes:
  - Domain filtering is supported to include or block specific websites
  - allowed_domains and blocked_domains are mutually exclusive (do not specify both)

IMPORTANT - Use the correct year in search queries:
  - The current month is %s. You MUST use this year when searching for recent information, documentation, or current events.
  - Example: If the user asks for "latest React docs", search for "React documentation" with the current year, NOT last year`, currentMonthYear)
}

func (t *WebSearchTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Query == "" {
		return fmt.Errorf("query must not be empty")
	}
	if len(in.Query) < 2 {
		return fmt.Errorf("query must be at least 2 characters")
	}
	if in.MaxResults < 0 {
		return fmt.Errorf("max_results must be non-negative")
	}
	if in.MaxResults > maxResultsCap {
		return fmt.Errorf("max_results exceeds maximum of %d", maxResultsCap)
	}
	// allowed_domains and blocked_domains are mutually exclusive.
	if len(in.AllowedDomains) > 0 && len(in.BlockedDomains) > 0 {
		return fmt.Errorf("allowed_domains and blocked_domains are mutually exclusive")
	}
	return nil
}

func (t *WebSearchTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Query == "" {
		return fmt.Errorf("query must not be empty")
	}
	return nil
}

func (t *WebSearchTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	if in.MaxResults <= 0 {
		in.MaxResults = defaultMaxResults
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		start := time.Now()

		// Emit progress: query_update
		emitProgress(uctx, &engine.ProgressData{
			ToolUseID:    uctx.ToolUseID,
			ProgressType: "web_search",
			WebSearch: &engine.WebSearchProgressData{
				Query: in.Query,
			},
		})

		hits, err := t.provider.Search(ctx, in.Query, in.MaxResults, in.AllowedDomains, in.BlockedDomains)
		if err != nil {
			ch <- errBlock(err.Error())
			return
		}

		elapsed := time.Since(start)

		// Emit progress: search_results_received
		emitProgress(uctx, &engine.ProgressData{
			ToolUseID:    uctx.ToolUseID,
			ProgressType: "web_search",
			WebSearch: &engine.WebSearchProgressData{
				Query:           in.Query,
				ResultsReceived: len(hits),
				DurationMs:      int(elapsed.Milliseconds()),
			},
		})

		out := Output{
			Query:           in.Query,
			Results:         hits,
			DurationSeconds: elapsed.Seconds(),
		}
		b, _ := json.Marshal(out)
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(b)}
	}()
	return ch, nil
}

// MapToolResultToBlockParam formats the search result for the model, matching
// claude-code-main's WebSearchTool.mapToolResultToToolResultBlockParam.
func (t *WebSearchTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text, ok := content.(string)
	if !ok {
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: ""}
	}

	// Try to parse as Output to produce the claude-code format.
	var out Output
	if json.Unmarshal([]byte(text), &out) == nil && len(out.Results) > 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Web search results for query: %q\n\n", out.Query))
		for _, hit := range out.Results {
			sb.WriteString(fmt.Sprintf("Title: %s\nURL: %s\n", hit.Title, hit.URL))
			if hit.Snippet != "" {
				sb.WriteString(fmt.Sprintf("Snippet: %s\n", hit.Snippet))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("REMINDER: You MUST include relevant sources from above in your response.")
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: sb.String()}
	}

	// Fallback: return as-is.
	return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: text}
}

// emitProgress safely calls OnToolProgress if available.
func emitProgress(uctx *tool.UseContext, p *engine.ProgressData) {
	if uctx != nil && uctx.OnToolProgress != nil {
		uctx.OnToolProgress(p)
	}
}

// ── DuckDuckGo provider ─────────────────────────────────────────────────────

// ddgProvider implements SearchProvider using the DuckDuckGo JSON API.
type ddgProvider struct {
	tool *WebSearchTool
}

func (p *ddgProvider) Name() string { return "DuckDuckGo" }

func (p *ddgProvider) Search(ctx context.Context, query string, maxResults int, allowedDomains, blockedDomains []string) ([]SearchHit, error) {
	params := url.Values{
		"q":       {query},
		"format":  {"json"},
		"no_html": {"1"},
	}
	reqURL := p.tool.baseURL + "/?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "AgentEngine/1.0")

	resp, err := p.tool.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}

	return parseDDGResponse(body, maxResults, allowedDomains, blockedDomains), nil
}

// parseDDGResponse extracts search hits from DuckDuckGo's JSON API response.
// Falls back to returning the raw body snippet if parsing fails.
func parseDDGResponse(body []byte, maxResults int, allowedDomains, blockedDomains []string) []SearchHit {
	var ddg struct {
		Abstract      string `json:"Abstract"`
		AbstractURL   string `json:"AbstractURL"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
		Results []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
	}

	if err := json.Unmarshal(body, &ddg); err != nil {
		// Fallback: return raw body as a single result.
		text := string(body)
		if len(text) > 5000 {
			text = text[:5000] + "\n[... truncated ...]"
		}
		return []SearchHit{{Title: "Raw search results", Snippet: text}}
	}

	allowed := toSet(allowedDomains)
	blocked := toSet(blockedDomains)

	var hits []SearchHit

	// Abstract result.
	if ddg.Abstract != "" && ddg.AbstractURL != "" {
		if domainMatchesFilter(ddg.AbstractURL, allowed, blocked) {
			hits = append(hits, SearchHit{
				Title:   extractTitle(ddg.Abstract),
				URL:     ddg.AbstractURL,
				Snippet: ddg.Abstract,
			})
		}
	}

	// Direct results.
	for _, r := range ddg.Results {
		if len(hits) >= maxResults {
			break
		}
		if r.FirstURL == "" {
			continue
		}
		if !domainMatchesFilter(r.FirstURL, allowed, blocked) {
			continue
		}
		hits = append(hits, SearchHit{
			Title:   extractTitle(r.Text),
			URL:     r.FirstURL,
			Snippet: r.Text,
		})
	}

	// Related topics.
	for _, r := range ddg.RelatedTopics {
		if len(hits) >= maxResults {
			break
		}
		if r.FirstURL == "" {
			continue
		}
		if !domainMatchesFilter(r.FirstURL, allowed, blocked) {
			continue
		}
		hits = append(hits, SearchHit{
			Title:   extractTitle(r.Text),
			URL:     r.FirstURL,
			Snippet: r.Text,
		})
	}

	return hits
}

// extractTitle returns the first ~80 chars of text as a title.
func extractTitle(text string) string {
	if text == "" {
		return "Search result"
	}
	if len(text) > 80 {
		return text[:80] + "\u2026"
	}
	return text
}

func toSet(ss []string) map[string]bool {
	if len(ss) == 0 {
		return nil
	}
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[strings.ToLower(s)] = true
	}
	return m
}

func domainMatchesFilter(rawURL string, allowed, blocked map[string]bool) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true // can't parse, let it through
	}
	host := strings.ToLower(u.Hostname())
	if len(allowed) > 0 {
		// Must match at least one allowed domain.
		for d := range allowed {
			if host == d || strings.HasSuffix(host, "."+d) {
				return true
			}
		}
		return false
	}
	if len(blocked) > 0 {
		for d := range blocked {
			if host == d || strings.HasSuffix(host, "."+d) {
				return false
			}
		}
	}
	return true
}

func errBlock(msg string) *engine.ContentBlock {
	return &engine.ContentBlock{Type: engine.ContentTypeText, Text: msg, IsError: true}
}
