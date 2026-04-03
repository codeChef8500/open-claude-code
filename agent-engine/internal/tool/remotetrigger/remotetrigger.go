package remotetrigger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// ────────────────────────────────────────────────────────────────────────────
// RemoteTriggerTool — triggers remote webhooks or HTTP endpoints.
// Used for CI/CD triggers, Slack notifications, external API calls, etc.
// Aligned with claude-code-main's remote trigger / webhook patterns.
// ────────────────────────────────────────────────────────────────────────────

const (
	defaultHTTPTimeout = 30 * time.Second
	maxResponseBody    = 50_000
)

// Input is the JSON input schema for RemoteTriggerTool.
type Input struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"` // GET, POST, PUT, DELETE
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Timeout int               `json:"timeout,omitempty"` // milliseconds
}

// RemoteTriggerTool makes HTTP requests to external endpoints.
type RemoteTriggerTool struct {
	tool.BaseTool
	// AllowedHosts restricts which hosts can be called. Empty means all allowed.
	AllowedHosts []string
	// Client is the HTTP client to use (nil uses default).
	Client *http.Client
}

func New() *RemoteTriggerTool {
	return &RemoteTriggerTool{}
}

func NewWithHosts(allowedHosts []string) *RemoteTriggerTool {
	return &RemoteTriggerTool{AllowedHosts: allowedHosts}
}

func (t *RemoteTriggerTool) Name() string           { return "RemoteTrigger" }
func (t *RemoteTriggerTool) UserFacingName() string { return "remote_trigger" }
func (t *RemoteTriggerTool) Description() string {
	return "Make HTTP requests to remote endpoints (webhooks, APIs, CI/CD triggers)."
}
func (t *RemoteTriggerTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *RemoteTriggerTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *RemoteTriggerTool) MaxResultSizeChars() int                  { return maxResponseBody }
func (t *RemoteTriggerTool) IsEnabled(uctx *tool.UseContext) bool     { return true }
func (t *RemoteTriggerTool) IsDestructive(_ json.RawMessage) bool     { return true }
func (t *RemoteTriggerTool) Aliases() []string {
	return []string{"remote_trigger", "webhook", "http_request"}
}

func (t *RemoteTriggerTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"url":{"type":"string","description":"The URL to send the request to."},
			"method":{"type":"string","description":"HTTP method.","enum":["GET","POST","PUT","DELETE","PATCH"]},
			"headers":{"type":"object","description":"HTTP headers as key-value pairs.","additionalProperties":{"type":"string"}},
			"body":{"type":"string","description":"Request body (for POST/PUT/PATCH)."},
			"timeout":{"type":"integer","description":"Timeout in milliseconds (default 30000)."}
		},
		"required":["url"]
	}`)
}

func (t *RemoteTriggerTool) Prompt(uctx *tool.UseContext) string {
	return `## RemoteTrigger
Make HTTP requests to external endpoints.
- Use for webhooks, CI/CD triggers, API calls
- Always verify the URL is correct before sending
- POST/PUT requests should include appropriate Content-Type headers
- Responses are truncated to 50K characters`
}

func (t *RemoteTriggerTool) CheckPermissions(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.URL == "" {
		return fmt.Errorf("url must not be empty")
	}
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		return fmt.Errorf("url must start with http:// or https://")
	}

	// Check allowed hosts if configured.
	if len(t.AllowedHosts) > 0 {
		host := extractHost(in.URL)
		allowed := false
		for _, h := range t.AllowedHosts {
			if strings.EqualFold(host, h) || strings.HasSuffix(host, "."+h) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("host %q is not in the allowed hosts list", host)
		}
	}

	return nil
}

func (t *RemoteTriggerTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	method := strings.ToUpper(in.Method)
	if method == "" {
		method = "GET"
	}

	timeout := defaultHTTPTimeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Millisecond
	}

	ch := make(chan *engine.ContentBlock, 4)
	go func() {
		defer close(ch)

		client := t.Client
		if client == nil {
			client = &http.Client{Timeout: timeout}
		}

		var bodyReader io.Reader
		if in.Body != "" {
			bodyReader = bytes.NewBufferString(in.Body)
		}

		req, err := http.NewRequestWithContext(ctx, method, in.URL, bodyReader)
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    "Request creation error: " + err.Error(),
				IsError: true,
			}
			return
		}

		// Set headers.
		for k, v := range in.Headers {
			req.Header.Set(k, v)
		}
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "agent-engine/1.0")
		}

		resp, err := client.Do(req)
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    "HTTP request error: " + err.Error(),
				IsError: true,
			}
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxResponseBody)+1))
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("HTTP %d - error reading body: %s", resp.StatusCode, err),
				IsError: true,
			}
			return
		}

		bodyStr := string(body)
		truncated := false
		if len(bodyStr) > maxResponseBody {
			bodyStr = bodyStr[:maxResponseBody]
			truncated = true
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("HTTP %d %s\n", resp.StatusCode, resp.Status))
		sb.WriteString(fmt.Sprintf("URL: %s %s\n", method, in.URL))
		sb.WriteString("---\n")
		sb.WriteString(bodyStr)
		if truncated {
			sb.WriteString("\n[... response truncated ...]")
		}

		isErr := resp.StatusCode >= 400
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    sb.String(),
			IsError: isErr,
		}
	}()
	return ch, nil
}

func (t *RemoteTriggerTool) GetActivityDescription(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return "Making HTTP request"
	}
	method := in.Method
	if method == "" {
		method = "GET"
	}
	url := in.URL
	if len(url) > 60 {
		url = url[:60] + "…"
	}
	return fmt.Sprintf("%s %s", method, url)
}

// extractHost extracts the hostname from a URL.
func extractHost(url string) string {
	// Strip scheme.
	if idx := strings.Index(url, "://"); idx >= 0 {
		url = url[idx+3:]
	}
	// Strip path.
	if idx := strings.Index(url, "/"); idx >= 0 {
		url = url[:idx]
	}
	// Strip port.
	if idx := strings.LastIndex(url, ":"); idx >= 0 {
		url = url[:idx]
	}
	return strings.ToLower(url)
}
