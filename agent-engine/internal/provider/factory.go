package provider

import "fmt"

// Config holds all settings needed to instantiate a Provider.
type Config struct {
	// "anthropic" or "openai"
	Type    string
	APIKey  string
	Model   string
	BaseURL string
}

// New creates a Provider from config. Returns an error for unknown types.
func New(cfg Config) (Provider, error) {
	switch cfg.Type {
	case "anthropic", "":
		return NewAnthropicProvider(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
	case "openai":
		return NewOpenAICompatProvider(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %q (supported: anthropic, openai)", cfg.Type)
	}
}
