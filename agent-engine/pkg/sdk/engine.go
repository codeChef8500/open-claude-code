// Package sdk is the public Go SDK entry point for the Agent Engine.
// Import it as: import "github.com/wall-ai/agent-engine/pkg/sdk"
package sdk

import (
	"context"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/tool/agentool"
	"github.com/wall-ai/agent-engine/internal/tool/askuser"
	"github.com/wall-ai/agent-engine/internal/tool/bash"
	"github.com/wall-ai/agent-engine/internal/tool/brief"
	"github.com/wall-ai/agent-engine/internal/tool/fileedit"
	"github.com/wall-ai/agent-engine/internal/tool/fileread"
	"github.com/wall-ai/agent-engine/internal/tool/filewrite"
	"github.com/wall-ai/agent-engine/internal/tool/glob"
	"github.com/wall-ai/agent-engine/internal/tool/grep"
	"github.com/wall-ai/agent-engine/internal/tool/notebookedit"
	"github.com/wall-ai/agent-engine/internal/tool/sendmessage"
	"github.com/wall-ai/agent-engine/internal/tool/sleep"
	"github.com/wall-ai/agent-engine/internal/tool/taskstop"
	"github.com/wall-ai/agent-engine/internal/tool/todo"
	"github.com/wall-ai/agent-engine/internal/tool/webfetch"
	"github.com/wall-ai/agent-engine/internal/tool/websearch"
	"github.com/wall-ai/agent-engine/internal/util"
)

// Engine is the public SDK handle for an agent session.
type Engine struct {
	inner *engine.Engine
}

// Options configures an Engine via functional options.
type Options struct {
	cfg engine.EngineConfig
}

// Option is a functional option for Engine creation.
type Option func(*Options)

func WithProvider(p string) Option    { return func(o *Options) { o.cfg.Provider = p } }
func WithAPIKey(k string) Option      { return func(o *Options) { o.cfg.APIKey = k } }
func WithModel(m string) Option       { return func(o *Options) { o.cfg.Model = m } }
func WithMaxTokens(n int) Option      { return func(o *Options) { o.cfg.MaxTokens = n } }
func WithWorkDir(d string) Option     { return func(o *Options) { o.cfg.WorkDir = d } }
func WithSessionID(id string) Option  { return func(o *Options) { o.cfg.SessionID = id } }
func WithAutoMode(b bool) Option      { return func(o *Options) { o.cfg.AutoMode = b } }
func WithVerbose(b bool) Option       { return func(o *Options) { o.cfg.Verbose = b } }
func WithBaseURL(u string) Option     { return func(o *Options) { o.cfg.BaseURL = u } }
func WithThinkingBudget(n int) Option { return func(o *Options) { o.cfg.ThinkingBudget = n } }
func WithCustomSystemPrompt(s string) Option {
	return func(o *Options) { o.cfg.CustomSystemPrompt = s }
}
func WithAppendSystemPrompt(s string) Option {
	return func(o *Options) { o.cfg.AppendSystemPrompt = s }
}

// New creates and returns a new Engine with the standard tool set.
func New(opts ...Option) (*Engine, error) {
	o := &Options{
		cfg: engine.EngineConfig{
			Provider:  util.GetString("provider"),
			Model:     util.GetString("model"),
			MaxTokens: util.GetInt("max_tokens"),
			Verbose:   util.GetBoolConfig("verbose"),
		},
	}
	for _, opt := range opts {
		opt(o)
	}

	if o.cfg.WorkDir == "" {
		return nil, fmt.Errorf("sdk.New: WorkDir is required (use sdk.WithWorkDir)")
	}

	prov, err := provider.New(provider.Config{
		Type:    o.cfg.Provider,
		APIKey:  o.cfg.APIKey,
		Model:   o.cfg.Model,
		BaseURL: o.cfg.BaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("sdk.New: provider: %w", err)
	}

	tools := defaultTools()

	inner, err := engine.New(o.cfg, prov, tools)
	if err != nil {
		return nil, fmt.Errorf("sdk.New: engine: %w", err)
	}

	return &Engine{inner: inner}, nil
}

// SessionID returns the unique session ID.
func (e *Engine) SessionID() string { return e.inner.SessionID() }

// SubmitMessage sends a user message and returns a streaming event channel.
func (e *Engine) SubmitMessage(ctx context.Context, text string) <-chan *engine.StreamEvent {
	return e.inner.SubmitMessage(ctx, engine.QueryParams{Text: text})
}

// SubmitMessageWithImages sends text and attached images.
func (e *Engine) SubmitMessageWithImages(ctx context.Context, text string, images []string) <-chan *engine.StreamEvent {
	return e.inner.SubmitMessage(ctx, engine.QueryParams{Text: text, Images: images})
}

// Close releases engine resources.
func (e *Engine) Close() error { return e.inner.Close() }

// defaultTools returns the standard set of tools registered for every engine.
func defaultTools() []tool.Tool {
	return []tool.Tool{
		bash.New(),
		fileread.New(),
		fileedit.New(),
		filewrite.New(),
		grep.New(),
		glob.New(),
		webfetch.New(),
		websearch.New("", ""),
		askuser.New(),
		todo.New(),
		sendmessage.New(),
		sleep.New(),
		taskstop.New(),
		notebookedit.New(),
		brief.New(),
		agentool.New(nil), // sub-agent runner wired at engine level
	}
}
