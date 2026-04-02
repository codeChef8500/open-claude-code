package webfetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	maxBodyBytes   = 5 * 1024 * 1024 // 5 MB
	maxOutputChars = 100_000
	httpTimeout    = 30 * time.Second
)

type Input struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt,omitempty"`
	// "html" | "markdown" | "text" — default "markdown"
	Format string `json:"format,omitempty"`
}

type WebFetchTool struct {
	tool.BaseTool
	client *http.Client
}

func New() *WebFetchTool {
	return &WebFetchTool{client: &http.Client{Timeout: httpTimeout}}
}

func (t *WebFetchTool) Name() string                      { return "WebFetch" }
func (t *WebFetchTool) UserFacingName() string            { return "web_fetch" }
func (t *WebFetchTool) Description() string               { return "Fetch the content of a web page." }
func (t *WebFetchTool) IsReadOnly(_ json.RawMessage) bool                  { return true }
func (t *WebFetchTool) IsConcurrencySafe(_ json.RawMessage) bool           { return true }
func (t *WebFetchTool) MaxResultSizeChars() int           { return maxOutputChars }
func (t *WebFetchTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *WebFetchTool) IsSearchOrRead(_ json.RawMessage) engine.SearchOrReadInfo { return engine.SearchOrReadInfo{IsSearch: true} }

func (t *WebFetchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"url":{"type":"string","description":"URL to fetch."},
			"prompt":{"type":"string","description":"Optional extraction prompt to focus the content."},
			"format":{"type":"string","enum":["html","markdown","text"],"description":"Output format. Default: markdown."}
		},
		"required":["url"]
	}`)
}

func (t *WebFetchTool) Prompt(_ *tool.UseContext) string { return "" }

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

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
		if err != nil {
			ch <- errBlock(err.Error())
			return
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 AgentEngine/1.0")

		resp, err := t.client.Do(req)
		if err != nil {
			ch <- errBlock(err.Error())
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		if err != nil {
			ch <- errBlock(err.Error())
			return
		}

		contentType := resp.Header.Get("Content-Type")
		var output string

		// PDF detection — extract text using ledongthuc/pdf.
		if strings.Contains(contentType, "application/pdf") || strings.HasSuffix(strings.ToLower(in.URL), ".pdf") {
			text, err := extractPDFText(body)
			if err != nil || strings.TrimSpace(text) == "" {
				ch <- &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: fmt.Sprintf("[PDF document — %d bytes; could not extract text: %v]", len(body), err),
				}
				return
			}
			if len(text) > maxOutputChars {
				text = text[:maxOutputChars] + "\n[... truncated ...]"
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: text}
			return
		}

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
				md, err := htmltomarkdown.ConvertString(string(body))
				if err != nil {
					output = stripHTML(string(body))
				} else {
					output = md
				}
			} else {
				output = string(body)
			}
		}

		if len(output) > maxOutputChars {
			output = output[:maxOutputChars] + "\n[... truncated ...]"
		}

		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: output}
	}()
	return ch, nil
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
