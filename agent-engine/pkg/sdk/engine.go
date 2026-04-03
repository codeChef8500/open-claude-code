// Package sdk is the public Go SDK entry point for the Agent Engine.
// Import it as: import "github.com/wall-ai/agent-engine/pkg/sdk"
package sdk

import (
	"context"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/memory"
	"github.com/wall-ai/agent-engine/internal/mode"
	"github.com/wall-ai/agent-engine/internal/permission"
	"github.com/wall-ai/agent-engine/internal/prompt"
	"github.com/wall-ai/agent-engine/internal/provider"
	"github.com/wall-ai/agent-engine/internal/session"
	"github.com/wall-ai/agent-engine/internal/skill"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/tool/agentool"
	"github.com/wall-ai/agent-engine/internal/toolset"
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

	// Build AgentTool with a real sub-agent runner.
	subRunner := buildSubAgentRunner(o.cfg, prov)
	tools := toolset.DefaultTools(subRunner)

	// Append discovered skills as tools.
	for _, s := range skill.DiscoverAll(o.cfg.WorkDir) {
		tools = append(tools, skill.NewSkillTool(s))
	}

	inner, err := engine.New(o.cfg, prov, tools)
	if err != nil {
		return nil, fmt.Errorf("sdk.New: engine: %w", err)
	}

	// Wire optional integrations (avoids circular imports at engine layer).
	inner.SetMemoryLoader(memory.NewAdapter())
	inner.SetSessionWriter(session.NewAdapter())
	inner.SetPromptBuilder(prompt.NewAdapter())
	inner.SetPermissionChecker(permission.NewAdapter())
	if o.cfg.AutoMode {
		inner.SetAutoModeClassifier(mode.NewClassifierAdapter(prov, nil))
	}

	return &Engine{inner: inner}, nil
}

// buildSubAgentRunner returns a SubAgentRunner that spins up a child engine
// with the same provider and a fresh session, runs the task to completion, and
// returns the concatenated assistant text.
func buildSubAgentRunner(cfg engine.EngineConfig, prov engine.ModelCaller) agentool.SubAgentRunner {
	return func(ctx context.Context, agentID, task string, input agentool.Input, uctx *tool.UseContext) (string, error) {
		childCfg := cfg
		childCfg.SessionID = agentID
		if uctx != nil && uctx.WorkDir != "" {
			childCfg.WorkDir = uctx.WorkDir
		}
		if input.SystemPrompt != "" {
			childCfg.CustomSystemPrompt = input.SystemPrompt
		}
		maxTurns := input.MaxTurns
		if maxTurns <= 0 {
			maxTurns = 50
		}
		childCfg.MaxTokens = cfg.MaxTokens

		subTools := toolset.DefaultTools(nil) // sub-agents cannot themselves spawn sub-agents (avoid infinite recursion)
		if len(input.AllowedTools) > 0 {
			allowed := make(map[string]bool, len(input.AllowedTools))
			for _, n := range input.AllowedTools {
				allowed[n] = true
			}
			var filtered []tool.Tool
			for _, t := range subTools {
				if allowed[t.Name()] {
					filtered = append(filtered, t)
				}
			}
			subTools = filtered
		}

		child, err := engine.New(childCfg, prov, subTools)
		if err != nil {
			return "", fmt.Errorf("sub-agent engine: %w", err)
		}
		child.SetMemoryLoader(memory.NewAdapter())
		child.SetPromptBuilder(prompt.NewAdapter())

		eventCh := child.SubmitMessage(ctx, engine.QueryParams{Text: task})
		var sb strings.Builder
		for ev := range eventCh {
			switch ev.Type {
			case engine.EventTextDelta:
				sb.WriteString(ev.Text)
			case engine.EventError:
				return "", fmt.Errorf("sub-agent error: %s", ev.Error)
			}
		}
		return sb.String(), nil
	}
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

// SkillNames returns names of all discovered skills for this engine.
func (e *Engine) SkillNames() []string {
	var names []string
	for _, s := range skill.DiscoverAll(e.inner.WorkDir()) {
		names = append(names, s.Meta.Name)
	}
	return names
}
