package webfetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/ledongthuc/pdf"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

// Ensure the import is used at compile time even if no html is encountered.
var _ = htmltomarkdown.ConvertString

const (
	maxBodyBytes   = 10 * 1024 * 1024 // 10 MB
	maxOutputChars = 100_000
	httpTimeout    = 60 * time.Second
	maxURLLength   = 2000
	cacheTTL       = 15 * time.Minute
)

type Input struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt,omitempty"`
	// "html" | "markdown" | "text" — default "markdown"
	Format string `json:"format,omitempty"`
}

// Output is the structured output of a WebFetch call.
type Output struct {
	Bytes      int    `json:"bytes"`
	Code       int    `json:"code"`
	CodeText   string `json:"codeText"`
	Result     string `json:"result"`
	DurationMs int64  `json:"durationMs"`
	URL        string `json:"url"`
}

// SideQuerier is the interface for auxiliary LLM calls (e.g. Haiku summaries).
// Matches provider.SideQuerier's Query method signature.
type SideQuerier interface {
	Query(ctx context.Context, prompt string, opts SideQueryOpts) (*SideQueryResult, error)
}

// SideQueryOpts mirrors provider.SideQueryOptions without importing the provider package.
type SideQueryOpts struct {
	Model        string
	MaxTokens    int
	SystemPrompt string
}

// SideQueryResult mirrors provider.SideQueryResult.
type SideQueryResult struct {
	Text string
}

type WebFetchTool struct {
	tool.BaseTool
	client      *http.Client
	cache       *FetchCache
	sideQuerier SideQuerier // optional: for Haiku content summaries
}

func New() *WebFetchTool {
	return &WebFetchTool{
		client: &http.Client{Timeout: httpTimeout},
		cache:  NewFetchCache(256),
	}
}

// SetSideQuerier enables Haiku-based content summarization for non-preapproved domains.
func (t *WebFetchTool) SetSideQuerier(sq SideQuerier) { t.sideQuerier = sq }

// ClearCache clears the internal URL cache.
func (t *WebFetchTool) ClearCache() { t.cache.Clear() }

func (t *WebFetchTool) Name() string                             { return "WebFetch" }
func (t *WebFetchTool) UserFacingName() string                   { return "Fetch" }
func (t *WebFetchTool) Description() string                      { return "Fetch the content of a web page." }
func (t *WebFetchTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *WebFetchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *WebFetchTool) MaxResultSizeChars() int                  { return maxOutputChars }
func (t *WebFetchTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *WebFetchTool) IsSearchOrRead(_ json.RawMessage) engine.SearchOrReadInfo {
	return engine.SearchOrReadInfo{IsSearch: true}
}

func (t *WebFetchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"url":{"type":"string","description":"The URL to fetch content from."},
			"prompt":{"type":"string","description":"The prompt to run on the fetched content."},
			"format":{"type":"string","enum":["html","markdown","text"],"description":"Output format. Default: markdown."}
		},
		"required":["url"]
	}`)
}

func (t *WebFetchTool) OutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"bytes":{"type":"integer","description":"Size of the fetched content in bytes."},
			"code":{"type":"integer","description":"HTTP response code."},
			"codeText":{"type":"string","description":"HTTP response code text."},
			"result":{"type":"string","description":"Processed result from applying the prompt to the content."},
			"durationMs":{"type":"integer","description":"Time taken to fetch and process the content."},
			"url":{"type":"string","description":"The URL that was fetched."}
		}
	}`)
}

func (t *WebFetchTool) GetToolUseSummary(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil || in.URL == "" {
		return ""
	}
	u := in.URL
	if len(u) > 80 {
		u = u[:80] + "…"
	}
	return u
}

func (t *WebFetchTool) GetActivityDescription(input json.RawMessage) string {
	s := t.GetToolUseSummary(input)
	if s != "" {
		return "Fetching " + s
	}
	return "Fetching web page"
}

func (t *WebFetchTool) ToAutoClassifierInput(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	if in.Prompt != "" {
		return in.URL + ": " + in.Prompt
	}
	return in.URL
}

func (t *WebFetchTool) Prompt(_ *tool.UseContext) string {
	return `IMPORTANT: WebFetch WILL FAIL for authenticated or private URLs. Before using this tool, check if the URL points to an authenticated service (e.g. Google Docs, Confluence, Jira, GitHub). If so, look for a specialized MCP tool that provides authenticated access.

- Fetches content from a specified URL and processes it
- Takes a URL and a prompt as input
- Fetches the URL content, converts HTML to markdown
- Returns the processed content
- Use this tool when you need to retrieve and analyze web content

Usage notes:
  - IMPORTANT: If an MCP-provided web fetch tool is available, prefer using that tool instead of this one, as it may have fewer restrictions.
  - The URL must be a fully-formed valid URL
  - HTTP URLs will be automatically upgraded to HTTPS
  - The prompt should describe what information you want to extract from the page
  - This tool is read-only and does not modify any files
  - Results may be summarized if the content is very large
  - Includes a self-cleaning 15-minute cache for faster responses when repeatedly accessing the same URL
  - When a URL redirects to a different host, the tool will inform you and provide the redirect URL. You should then make a new WebFetch request with the redirect URL to fetch the content.
  - For GitHub URLs, prefer using the gh CLI via Bash instead (e.g., gh pr view, gh issue view, gh api).
  - Supports HTML (converted to markdown), plain text, and PDF content`
}

func (t *WebFetchTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.URL == "" {
		return fmt.Errorf("url must not be empty")
	}
	if len(in.URL) > maxURLLength {
		return fmt.Errorf("URL exceeds maximum length of %d characters", maxURLLength)
	}
	u, err := url.Parse(in.URL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: could not be parsed", in.URL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	// Reject URLs with embedded credentials.
	if u.User != nil {
		return fmt.Errorf("URL must not contain username or password")
	}
	// Hostname must have at least 2 segments (e.g. "example.com").
	parts := strings.Split(u.Hostname(), ".")
	if len(parts) < 2 {
		return fmt.Errorf("URL hostname must contain at least two segments")
	}
	if in.Format != "" && in.Format != "html" && in.Format != "markdown" && in.Format != "text" {
		return fmt.Errorf("format must be \"html\", \"markdown\", or \"text\"")
	}
	return nil
}

func (t *WebFetchTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.URL == "" {
		return fmt.Errorf("url must not be empty")
	}
	// SSRF guard: rejects private/loopback addresses and metadata endpoints.
	if err := util.CheckSSRF(in.URL); err != nil {
		return err
	}
	// Domain blocklist check.
	if u, err := url.Parse(in.URL); err == nil {
		if msg := CheckDomainBlocklist(u.Hostname()); msg != "" {
			return fmt.Errorf("%s", msg)
		}
	}
	return nil
}

func (t *WebFetchTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		start := time.Now()

		// Emit progress: connecting
		emitFetchProgress(uctx, in.URL, "connecting", 0, 0)

		// --- Cache lookup ---
		if cached := t.cache.Get(in.URL); cached != nil {
			result := cached.Body
			if len(result) > maxOutputChars {
				result = result[:maxOutputChars] + "\n[... truncated ...]"
			}
			out := Output{
				Bytes:      len(cached.Body),
				Code:       cached.StatusCode,
				CodeText:   http.StatusText(cached.StatusCode),
				Result:     result,
				DurationMs: time.Since(start).Milliseconds(),
				URL:        in.URL,
			}
			b, _ := json.Marshal(out)
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(b)}
			return
		}

		// --- Upgrade http → https ---
		fetchURL := in.URL
		if u, err := url.Parse(fetchURL); err == nil && u.Scheme == "http" {
			u.Scheme = "https"
			fetchURL = u.String()
		}

		// --- Fetch with redirect handling ---
		headers := map[string]string{
			"User-Agent": "Mozilla/5.0 AgentEngine/1.0",
		}
		resp, body, redir, err := fetchWithRedirects(t.client, fetchURL, headers, maxBodyBytes)
		if err != nil {
			ch <- errBlock(err.Error())
			return
		}

		// --- Cross-host redirect detected ---
		if redir != nil {
			statusText := http.StatusText(redir.StatusCode)
			msg := fmt.Sprintf("REDIRECT DETECTED: The URL redirects to a different host.\n\n"+
				"Original URL: %s\nRedirect URL: %s\nStatus: %d %s\n\n"+
				"To complete your request, please use WebFetch again with:\n- url: %q\n- prompt: %q",
				redir.OriginalURL, redir.RedirectURL, redir.StatusCode, statusText,
				redir.RedirectURL, in.Prompt)
			out := Output{
				Bytes:      len(msg),
				Code:       redir.StatusCode,
				CodeText:   statusText,
				Result:     msg,
				DurationMs: time.Since(start).Milliseconds(),
				URL:        in.URL,
			}
			b, _ := json.Marshal(out)
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(b)}
			return
		}

		contentType := ""
		statusCode := 0
		statusText := ""
		if resp != nil {
			contentType = resp.Header.Get("Content-Type")
			statusCode = resp.StatusCode
			statusText = http.StatusText(resp.StatusCode)
		}

		var output string

		// PDF detection — extract text using ledongthuc/pdf.
		if strings.Contains(contentType, "application/pdf") || strings.HasSuffix(strings.ToLower(in.URL), ".pdf") {
			text, pdfErr := extractPDFText(body)
			if pdfErr != nil || strings.TrimSpace(text) == "" {
				output = fmt.Sprintf("[PDF document — %d bytes; could not extract text: %v]", len(body), pdfErr)
			} else {
				output = text
			}
		} else {
			format := in.Format
			if format == "" {
				format = "markdown"
			}
			switch format {
			case "html":
				output = string(body)
			case "text":
				output = stripHTML(string(body))
			default: // markdown
				if strings.Contains(contentType, "text/html") {
					md, mdErr := htmltomarkdown.ConvertString(string(body))
					if mdErr != nil {
						output = stripHTML(string(body))
					} else {
						output = md
					}
				} else {
					output = string(body)
				}
			}
		}

		// --- Emit progress: downloading ---
		emitFetchProgress(uctx, in.URL, "downloading", len(body), statusCode)

		// --- Store in cache ---
		t.cache.Set(in.URL, &CacheEntry{
			Body:        output,
			ContentType: contentType,
			StatusCode:  statusCode,
			FetchedAt:   time.Now(),
			TTL:         cacheTTL,
		})

		// --- Haiku summary for non-preapproved domains ---
		if t.sideQuerier != nil && in.Prompt != "" && !IsPreapprovedURL(in.URL) && len(output) > 0 {
			emitFetchProgress(uctx, in.URL, "processing", len(body), statusCode)
			summarized, err := applyPromptToContent(ctx, t.sideQuerier, in.Prompt, output)
			if err == nil && summarized != "" {
				output = summarized
			}
		}

		if len(output) > maxOutputChars {
			output = output[:maxOutputChars] + "\n[... truncated ...]"
		}

		out := Output{
			Bytes:      len(body),
			Code:       statusCode,
			CodeText:   statusText,
			Result:     output,
			DurationMs: time.Since(start).Milliseconds(),
			URL:        in.URL,
		}
		b, _ := json.Marshal(out)
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(b)}
	}()
	return ch, nil
}

// MapToolResultToBlockParam returns the result text directly for the model,
// matching claude-code-main's WebFetchTool.mapToolResultToToolResultBlockParam.
func (t *WebFetchTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text, ok := content.(string)
	if !ok {
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: ""}
	}
	// Try to extract just the result text from JSON Output.
	var out Output
	if json.Unmarshal([]byte(text), &out) == nil && out.Result != "" {
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: out.Result}
	}
	return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: text}
}

// applyPromptToContent uses a SideQuerier (Haiku) to summarize fetched content.
func applyPromptToContent(ctx context.Context, sq SideQuerier, prompt, content string) (string, error) {
	// Cap content to avoid huge prompts.
	const maxContentForSummary = 50_000
	if len(content) > maxContentForSummary {
		content = content[:maxContentForSummary] + "\n[... truncated for summary ...]\n"
	}

	sysPrompt := "You are a content extraction assistant. Extract the requested information from the provided web page content. Be concise and accurate."
	userMsg := fmt.Sprintf("Web page content:\n\n%s\n\n---\nUser request: %s", content, prompt)

	result, err := sq.Query(ctx, userMsg, SideQueryOpts{
		Model:        "claude-haiku-3-5",
		MaxTokens:    4096,
		SystemPrompt: sysPrompt,
	})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// emitFetchProgress safely calls OnToolProgress with WebFetch-specific data.
func emitFetchProgress(uctx *tool.UseContext, fetchURL, phase string, bytesRead, statusCode int) {
	if uctx == nil || uctx.OnToolProgress == nil {
		return
	}
	uctx.OnToolProgress(&engine.ProgressData{
		ToolUseID:    uctx.ToolUseID,
		ProgressType: "web_fetch",
		WebFetch: &engine.WebFetchProgressData{
			URL:        fetchURL,
			Phase:      phase,
			BytesRead:  bytesRead,
			StatusCode: statusCode,
		},
	})
}

func errBlock(msg string) *engine.ContentBlock {
	return &engine.ContentBlock{Type: engine.ContentTypeText, Text: msg, IsError: true}
}

// extractPDFText extracts plain text from a PDF byte slice using ledongthuc/pdf.
func extractPDFText(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pdf reader: %w", err)
	}
	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		content, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func stripHTML(s string) string {
	// Minimal HTML tag stripper for non-html-to-markdown fallback.
	inTag := false
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}
