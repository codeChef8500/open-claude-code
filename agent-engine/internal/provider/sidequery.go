package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// SideQueryOptions configures a lightweight one-shot LLM call.
type SideQueryOptions struct {
	// Model to use (defaults to claude-haiku-3-5 for cost efficiency).
	Model string
	// MaxTokens for the response (default 1024).
	MaxTokens int
	// SystemPrompt is prepended before the user message.
	SystemPrompt string
	// Temperature (0.0–1.0). 0 means use the model default.
	Temperature float64
	// JSONMode requests structured JSON output when true.
	JSONMode bool
}

// SideQueryResult is the response from a side query.
type SideQueryResult struct {
	// Text is the raw text content of the response.
	Text string
	// Parsed holds the JSON-decoded result when JSONMode was true and
	// the response was valid JSON.
	Parsed interface{}
	// InputTokens / OutputTokens from the response.
	InputTokens  int
	OutputTokens int
}

// SideQuerier performs lightweight one-shot LLM queries using the Anthropic API.
// It is intended for permission classification, summarisation helpers, and other
// auxiliary calls that don't need the full engine loop.
type SideQuerier struct {
	client anthropic.Client
	apiKey string
}

// NewSideQuerier creates a SideQuerier using the given API key.
func NewSideQuerier(apiKey string) *SideQuerier {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &SideQuerier{client: client, apiKey: apiKey}
}

// Query executes a single-turn query and returns the result.
func (sq *SideQuerier) Query(ctx context.Context, prompt string, opts SideQueryOptions) (*SideQueryResult, error) {
	model := opts.Model
	if model == "" {
		model = "claude-haiku-3-5"
	}
	maxTok := opts.MaxTokens
	if maxTok <= 0 {
		maxTok = 1024
	}

	msgParams := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTok),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}
	if opts.SystemPrompt != "" {
		msgParams.System = []anthropic.TextBlockParam{
			{Text: opts.SystemPrompt},
		}
	}

	resp, err := sq.client.Messages.New(ctx, msgParams)
	if err != nil {
		return nil, fmt.Errorf("side query: %w", err)
	}

	var text string
	for _, block := range resp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	result := &SideQueryResult{
		Text:         text,
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
	}

	if opts.JSONMode && text != "" {
		// Attempt to extract JSON from the response (handles markdown code fences).
		jsonStr := extractJSON(text)
		var parsed interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			result.Parsed = parsed
		}
	}

	return result, nil
}

// QueryJSON is a convenience wrapper that calls Query with JSONMode=true and
// unmarshals the result into out.
func (sq *SideQuerier) QueryJSON(ctx context.Context, prompt string, opts SideQueryOptions, out interface{}) error {
	opts.JSONMode = true
	res, err := sq.Query(ctx, prompt, opts)
	if err != nil {
		return err
	}
	jsonStr := extractJSON(res.Text)
	if err := json.Unmarshal([]byte(jsonStr), out); err != nil {
		return fmt.Errorf("side query json decode: %w (raw: %.200s)", err, res.Text)
	}
	return nil
}

// extractJSON strips optional markdown code fences and returns the raw JSON content.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip ```json ... ``` or ``` ... ```
	for _, fence := range []string{"```json", "```"} {
		if strings.HasPrefix(s, fence) {
			s = strings.TrimPrefix(s, fence)
			if idx := strings.LastIndex(s, "```"); idx >= 0 {
				s = s[:idx]
			}
			s = strings.TrimSpace(s)
			break
		}
	}
	return s
}
