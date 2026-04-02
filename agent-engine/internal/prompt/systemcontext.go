package prompt

import (
	"fmt"
	"strings"
)

// SystemContext holds all contextual information assembled for the system prompt.
type SystemContext struct {
	Env        *EnvContext
	Git        GitContext
	User       *UserContext
	Memory     string // pre-merged CLAUDE.md content
	SessionID  string
	Model      string
	TurnCount  int
	TokenUsage *TokenUsageContext
}

// TokenUsageContext carries token budget info for system prompt injection.
type TokenUsageContext struct {
	InputTokens       int
	OutputTokens      int
	ContextWindowSize int
	UsedFraction      float64
}

// BuildSystemContext assembles a full SystemContext from components.
func BuildSystemContext(workDir string, opts ...SystemContextOption) *SystemContext {
	ctx := &SystemContext{
		Env: BuildEnvContext(workDir),
		Git: DetectGitContext(workDir),
	}
	for _, o := range opts {
		o(ctx)
	}
	// Wire git info into env context.
	ctx.Env.IsGitRepo = ctx.Git.IsRepo
	ctx.Env.GitBranch = ctx.Git.Branch
	return ctx
}

// SystemContextOption configures a SystemContext.
type SystemContextOption func(*SystemContext)

// WithUser sets user context.
func WithUser(u *UserContext) SystemContextOption {
	return func(c *SystemContext) { c.User = u }
}

// WithMemory sets memory content.
func WithMemory(content string) SystemContextOption {
	return func(c *SystemContext) { c.Memory = content }
}

// WithSession sets session metadata.
func WithSession(id, model string, turns int) SystemContextOption {
	return func(c *SystemContext) {
		c.SessionID = id
		c.Model = model
		c.TurnCount = turns
	}
}

// WithTokenUsage sets token usage info.
func WithTokenUsage(input, output, contextWindow int) SystemContextOption {
	return func(c *SystemContext) {
		frac := 0.0
		if contextWindow > 0 {
			frac = float64(input+output) / float64(contextWindow)
		}
		c.TokenUsage = &TokenUsageContext{
			InputTokens:       input,
			OutputTokens:      output,
			ContextWindowSize: contextWindow,
			UsedFraction:      frac,
		}
	}
}

// Render formats the full system context as a prompt section.
func (sc *SystemContext) Render() string {
	var parts []string

	// Environment.
	if sc.Env != nil {
		parts = append(parts, sc.Env.Render())
	}

	// Git context.
	if sc.Git.IsRepo {
		parts = append(parts, sc.Git.Render())
	}

	// User context.
	if sc.User != nil {
		uc := InjectUserContext(sc.User)
		if uc != "" {
			parts = append(parts, uc)
		}
	}

	// Session info.
	if sc.SessionID != "" || sc.Model != "" {
		var sb strings.Builder
		sb.WriteString("<session>\n")
		if sc.SessionID != "" {
			sb.WriteString(fmt.Sprintf("session_id: %s\n", sc.SessionID))
		}
		if sc.Model != "" {
			sb.WriteString(fmt.Sprintf("model: %s\n", sc.Model))
		}
		if sc.TurnCount > 0 {
			sb.WriteString(fmt.Sprintf("turn: %d\n", sc.TurnCount))
		}
		sb.WriteString("</session>")
		parts = append(parts, sb.String())
	}

	// Token usage warning.
	if sc.TokenUsage != nil {
		used := sc.TokenUsage.InputTokens + sc.TokenUsage.OutputTokens
		level := GetTokenWarningLevel(used, sc.TokenUsage.ContextWindowSize)
		if msg := TokenWarningMessage(level, used, sc.TokenUsage.ContextWindowSize); msg != "" {
			parts = append(parts, msg)
		}
	}

	return strings.Join(parts, "\n\n")
}
