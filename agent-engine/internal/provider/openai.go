package provider

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
)

const openAIDefaultBaseURL = "https://api.openai.com/v1"

// OpenAIProvider implements engine.ModelCaller for OpenAI-compatible APIs.
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAIProvider creates a caller for the OpenAI API.
// If baseURL is empty, api.openai.com is used.
func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = openAIDefaultBaseURL
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

// Name returns the provider identifier.
func (p *OpenAIProvider) Name() string { return "openai" }

// CallModel sends a chat completion request and streams events back.
func (p *OpenAIProvider) CallModel(ctx context.Context, params engine.CallParams) (<-chan *engine.StreamEvent, error) {
	body, err := p.buildRequest(params)
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, string(body))
	}

	out := make(chan *engine.StreamEvent, 64)
	go p.streamResponse(ctx, resp.Body, out)
	return out, nil
}

// buildRequest converts CallParams to an OpenAI chat completions request JSON.
func (p *OpenAIProvider) buildRequest(params engine.CallParams) ([]byte, error) {
	type oaiMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]oaiMsg, 0, len(params.Messages)+1)
	if params.SystemPrompt != "" {
		msgs = append(msgs, oaiMsg{Role: "system", Content: params.SystemPrompt})
	}
	for _, m := range params.Messages {
		role := string(m.Role)
		if role == "assistant" || role == "user" {
			var sb strings.Builder
			for _, b := range m.Content {
				if b.Type == engine.ContentTypeText {
					sb.WriteString(b.Text)
				}
			}
			msgs = append(msgs, oaiMsg{Role: role, Content: sb.String()})
		}
	}

	payload := map[string]interface{}{
		"model":    params.Model,
		"messages": msgs,
		"stream":   true,
	}
	if params.MaxTokens > 0 {
		payload["max_tokens"] = params.MaxTokens
	}
	return json.Marshal(payload)
}

// streamResponse reads an OpenAI SSE stream and emits engine.StreamEvents.
func (p *OpenAIProvider) streamResponse(ctx context.Context, body io.ReadCloser, out chan<- *engine.StreamEvent) {
	defer body.Close()
	defer close(out)

	sseCh := StreamReader(ctx, body)
	for raw := range sseCh {
		data, ok := raw["data"]
		if !ok || data == "[DONE]" {
			out <- &engine.StreamEvent{Type: engine.EventDone}
			return
		}
		ev := p.parseChunk(data)
		if ev != nil {
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}
}

func (p *OpenAIProvider) parseChunk(data string) *engine.StreamEvent {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil
	}
	if len(chunk.Choices) > 0 {
		text := chunk.Choices[0].Delta.Content
		if text != "" {
			return &engine.StreamEvent{Type: engine.EventTextDelta, Text: text}
		}
		if chunk.Choices[0].FinishReason == "stop" {
			return &engine.StreamEvent{Type: engine.EventDone}
		}
	}
	if chunk.Usage != nil {
		return &engine.StreamEvent{
			Type: engine.EventUsage,
			Usage: &engine.UsageStats{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
			},
		}
	}
	return nil
}
