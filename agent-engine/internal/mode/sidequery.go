package mode

import (
	"context"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

// SideQueryOptions configures an independent LLM API call that does not
// affect the main conversation context.
type SideQueryOptions struct {
	Model        string
	MaxTokens    int
	SystemPrompt string
	UserMessage  string
	// UsePromptCache sets cache_control on the system block.
	UsePromptCache bool
}

// SideQueryResult is the plain-text response from a side query.
type SideQueryResult struct {
	Text  string
	Usage engine.UsageStats
}

// RunSideQuery executes an independent single-turn API call on the given
// provider. It does not share any state with the main query loop.
func RunSideQuery(ctx context.Context, prov provider.Provider, opts SideQueryOptions) (*SideQueryResult, error) {
	if opts.Model == "" {
		opts.Model = "claude-haiku-4-5"
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 1024
	}

	messages := []*engine.Message{
		{
			Role:    engine.RoleUser,
			Content: []*engine.ContentBlock{{Type: engine.ContentTypeText, Text: opts.UserMessage}},
		},
	}

	params := provider.CallParams{
		Model:          opts.Model,
		MaxTokens:      opts.MaxTokens,
		SystemPrompt:   opts.SystemPrompt,
		Messages:       messages,
		UsePromptCache: opts.UsePromptCache,
	}

	eventCh, err := prov.CallModel(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("side query: %w", err)
	}

	var result SideQueryResult
	for ev := range eventCh {
		switch ev.Type {
		case engine.EventTextDelta:
			result.Text += ev.Text
		case engine.EventUsage:
			if ev.Usage != nil {
				result.Usage = *ev.Usage
			}
		case engine.EventError:
			return nil, fmt.Errorf("side query provider error: %s", ev.Error)
		}
	}

	return &result, nil
}

// IsEnvTruthy is re-exported from util for mode-package use without a cycle.
func IsEnvTruthy(value string) bool {
	switch value {
	case "1", "true", "yes", "on", "TRUE", "YES", "ON":
		return true
	}
	return false
}
