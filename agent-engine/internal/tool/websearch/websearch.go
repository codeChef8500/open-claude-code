package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

const (
	maxResults    = 10
	httpTimeout   = 15 * time.Second
)

type Input struct {
	Query   string `json:"query"`
	MaxResults int `json:"max_results,omitempty"`
}

// WebSearchTool uses a configurable search backend (default: DuckDuckGo JSON API).
type WebSearchTool struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func New(apiKey, baseURL string) *WebSearchTool {
	if baseURL == "" {
		baseURL = "https://api.duckduckgo.com"
	}
	return &WebSearchTool{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: httpTimeout},
	}
}

func (t *WebSearchTool) Name() string            { return "WebSearch" }
func (t *WebSearchTool) UserFacingName() string  { return "web_search" }
func (t *WebSearchTool) Description() string     { return "Search the web for information." }
func (t *WebSearchTool) IsReadOnly() bool        { return true }
func (t *WebSearchTool) IsConcurrencySafe() bool { return true }
func (t *WebSearchTool) MaxResultSizeChars() int { return 50_000 }
func (t *WebSearchTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *WebSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"query":{"type":"string","description":"Search query."},
			"max_results":{"type":"integer","description":"Maximum number of results (default 10)."}
		},
		"required":["query"]
	}`)
}

func (t *WebSearchTool) Prompt(_ *tool.UseContext) string { return "" }

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

func (t *WebSearchTool) Call(ctx context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	if in.MaxResults <= 0 {
		in.MaxResults = maxResults
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		params := url.Values{
			"q":       {in.Query},
			"format":  {"json"},
			"no_html": {"1"},
		}
		reqURL := t.baseURL + "/?" + params.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			ch <- errBlock(err.Error())
			return
		}
		req.Header.Set("User-Agent", "AgentEngine/1.0")

		resp, err := t.client.Do(req)
		if err != nil {
			ch <- errBlock(err.Error())
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		if err != nil {
			ch <- errBlock(err.Error())
			return
		}

		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(body)}
	}()
	return ch, nil
}

func errBlock(msg string) *engine.ContentBlock {
	return &engine.ContentBlock{Type: engine.ContentTypeText, Text: msg, IsError: true}
}
