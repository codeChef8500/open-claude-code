package command

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// Auth & Account command deep implementations.
// Aligned with claude-code-main commands/login, logout, usage, extra-usage, passes.
// ──────────────────────────────────────────────────────────────────────────────

// ─── /login deep implementation ─────────────────────────────────────────────
// Aligned with claude-code-main commands/login/login.tsx.

// LoginViewData is the structured data for the login TUI component.
type LoginViewData struct {
	IsAuthenticated bool   `json:"is_authenticated"`
	AuthURL         string `json:"auth_url,omitempty"`
	Message         string `json:"message,omitempty"`
	Error           string `json:"error,omitempty"`
}

// DeepLoginCommand handles OAuth2 authentication flow.
type DeepLoginCommand struct{ BaseCommand }

func (c *DeepLoginCommand) Name() string                  { return "login" }
func (c *DeepLoginCommand) Description() string           { return "Sign in to your Anthropic account" }
func (c *DeepLoginCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepLoginCommand) IsEnabled(ectx *ExecContext) bool {
	// Hidden when using 3P services (Bedrock/Vertex).
	if ectx != nil && ectx.Services != nil && ectx.Services.Auth != nil {
		return !ectx.Services.Auth.IsUsing3PServices()
	}
	return true
}
func (c *DeepLoginCommand) Availability() []CommandAvailability {
	return []CommandAvailability{AvailabilityConsole}
}

func (c *DeepLoginCommand) ExecuteInteractive(ctx context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &LoginViewData{}

	if ectx != nil && ectx.Services != nil && ectx.Services.Auth != nil {
		auth := ectx.Services.Auth
		data.IsAuthenticated = auth.IsAuthenticated()

		if data.IsAuthenticated {
			data.Message = "Already logged in."
		} else {
			// Initiate login flow.
			if err := auth.Login(ctx); err != nil {
				data.Error = fmt.Sprintf("Login failed: %v", err)
			} else {
				data.Message = "Login successful."
				data.IsAuthenticated = true
			}
		}
	} else {
		data.Error = "Auth service not available."
	}

	return &InteractiveResult{
		Component: "login",
		Data:      data,
	}, nil
}

// ─── /logout deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/logout/logout.tsx.

// DeepLogoutCommand handles sign-out and token clearing.
type DeepLogoutCommand struct{ BaseCommand }

func (c *DeepLogoutCommand) Name() string                  { return "logout" }
func (c *DeepLogoutCommand) Description() string           { return "Sign out of your Anthropic account" }
func (c *DeepLogoutCommand) Type() CommandType             { return CommandTypeLocal }
func (c *DeepLogoutCommand) IsEnabled(ectx *ExecContext) bool {
	if ectx != nil && ectx.Services != nil && ectx.Services.Auth != nil {
		return !ectx.Services.Auth.IsUsing3PServices()
	}
	return true
}
func (c *DeepLogoutCommand) Availability() []CommandAvailability {
	return []CommandAvailability{AvailabilityConsole}
}

func (c *DeepLogoutCommand) Execute(ctx context.Context, _ []string, ectx *ExecContext) (string, error) {
	if ectx == nil || ectx.Services == nil || ectx.Services.Auth == nil {
		return "Auth service not available.", nil
	}

	auth := ectx.Services.Auth
	if !auth.IsAuthenticated() {
		return "Not logged in.", nil
	}

	if err := auth.Logout(ctx); err != nil {
		return fmt.Sprintf("Logout failed: %v", err), nil
	}

	// Clear caches after logout.
	if ectx.Services.Cache != nil {
		ectx.Services.Cache.ClearAll()
	}

	return "Logged out successfully.", nil
}

// ─── /usage deep implementation ─────────────────────────────────────────────
// Aligned with claude-code-main commands/usage/usage.tsx.

// UsageViewData is the structured data for the usage display TUI component.
type UsageViewData struct {
	IsSubscriber    bool    `json:"is_subscriber"`
	UsedTokens      int     `json:"used_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	UsedCost        float64 `json:"used_cost"`
	RemainingCost   float64 `json:"remaining_cost"`
	UsagePercentage float64 `json:"usage_percentage"`
	PlanName        string  `json:"plan_name,omitempty"`
	ResetDate       string  `json:"reset_date,omitempty"`
	Error           string  `json:"error,omitempty"`
}

// DeepUsageCommand shows API usage statistics.
type DeepUsageCommand struct{ BaseCommand }

func (c *DeepUsageCommand) Name() string                  { return "usage" }
func (c *DeepUsageCommand) Description() string           { return "Show API usage and quota" }
func (c *DeepUsageCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepUsageCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepUsageCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &UsageViewData{}
	if ectx != nil {
		data.UsedCost = ectx.CostUSD
		data.UsedTokens = ectx.TotalTokens
	}
	if ectx != nil && ectx.Services != nil && ectx.Services.Auth != nil {
		data.IsSubscriber = ectx.Services.Auth.IsClaudeAISubscriber()
	}
	return &InteractiveResult{
		Component: "usage",
		Data:      data,
	}, nil
}

// ─── /extra-usage deep implementation ───────────────────────────────────────
// Aligned with claude-code-main commands/extra-usage/extra-usage.tsx.

// ExtraUsageViewData is the structured data for extra usage purchase options.
type ExtraUsageViewData struct {
	IsSubscriber bool   `json:"is_subscriber"`
	PurchaseURL  string `json:"purchase_url,omitempty"`
	Message      string `json:"message,omitempty"`
}

// DeepExtraUsageCommand shows extra usage purchase options (claude-ai only).
type DeepExtraUsageCommand struct{ BaseCommand }

func (c *DeepExtraUsageCommand) Name() string        { return "extra-usage" }
func (c *DeepExtraUsageCommand) Description() string { return "Purchase additional usage credits" }
func (c *DeepExtraUsageCommand) Type() CommandType   { return CommandTypeInteractive }
func (c *DeepExtraUsageCommand) Availability() []CommandAvailability {
	return []CommandAvailability{AvailabilityClaudeAI}
}
func (c *DeepExtraUsageCommand) IsEnabled(ectx *ExecContext) bool {
	if ectx != nil && ectx.Services != nil && ectx.Services.Auth != nil {
		return ectx.Services.Auth.IsClaudeAISubscriber()
	}
	return false
}

func (c *DeepExtraUsageCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &ExtraUsageViewData{
		PurchaseURL: "https://claude.ai/settings/billing",
	}
	if ectx != nil && ectx.Services != nil && ectx.Services.Auth != nil {
		data.IsSubscriber = ectx.Services.Auth.IsClaudeAISubscriber()
	}
	return &InteractiveResult{
		Component: "extra-usage",
		Data:      data,
	}, nil
}

// ExtraUsageNonInteractiveCommand is the non-interactive fallback.
type ExtraUsageNonInteractiveCommand struct{ BaseCommand }

func (c *ExtraUsageNonInteractiveCommand) Name() string        { return "extra-usage-text" }
func (c *ExtraUsageNonInteractiveCommand) Description() string { return "Show extra usage info (text mode)" }
func (c *ExtraUsageNonInteractiveCommand) Type() CommandType   { return CommandTypeLocal }
func (c *ExtraUsageNonInteractiveCommand) IsHidden() bool      { return true }
func (c *ExtraUsageNonInteractiveCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ExtraUsageNonInteractiveCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "Visit https://claude.ai/settings/billing to purchase additional usage.", nil
}

// ─── /passes deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/passes/passes.tsx.

// PassesViewData is the structured data for the passes display.
type PassesViewData struct {
	Passes  []PassInfo `json:"passes,omitempty"`
	Message string     `json:"message,omitempty"`
}

// PassInfo describes an activated pass.
type PassInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // "daily", "weekly", "monthly"
	ExpiresAt string `json:"expires_at,omitempty"`
	Active    bool   `json:"active"`
}

// DeepPassesCommand shows activated passes.
type DeepPassesCommand struct{ BaseCommand }

func (c *DeepPassesCommand) Name() string                  { return "passes" }
func (c *DeepPassesCommand) Description() string           { return "Show activated passes" }
func (c *DeepPassesCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepPassesCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepPassesCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	data := &PassesViewData{
		Message: "No active passes.",
	}
	return &InteractiveResult{
		Component: "passes",
		Data:      data,
	}, nil
}

// ─── /usage-report ──────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/usage-report (local).

// UsageReportCommand generates a detailed usage report.
type UsageReportCommand struct{ BaseCommand }

func (c *UsageReportCommand) Name() string                  { return "usage-report" }
func (c *UsageReportCommand) Description() string           { return "Generate a detailed usage report" }
func (c *UsageReportCommand) Type() CommandType             { return CommandTypeLocal }
func (c *UsageReportCommand) IsHidden() bool                { return true }
func (c *UsageReportCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *UsageReportCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "No session context available.", nil
	}

	var sb strings.Builder
	sb.WriteString("## Usage Report\n\n")
	sb.WriteString(fmt.Sprintf("Session: %s\n", ectx.SessionID))
	sb.WriteString(fmt.Sprintf("Model: %s\n", ectx.Model))
	sb.WriteString(fmt.Sprintf("Turns: %d\n", ectx.TurnCount))
	sb.WriteString(fmt.Sprintf("Total tokens: %d\n", ectx.TotalTokens))
	sb.WriteString(fmt.Sprintf("Cost: $%.4f\n", ectx.CostUSD))

	if ectx.ContextStats != nil {
		sb.WriteString(fmt.Sprintf("\nContext Window:\n"))
		sb.WriteString(fmt.Sprintf("  Input:  %d / %d tokens\n", ectx.ContextStats.InputTokens, ectx.ContextStats.ContextWindowSize))
		sb.WriteString(fmt.Sprintf("  Output: %d tokens\n", ectx.ContextStats.OutputTokens))
		sb.WriteString(fmt.Sprintf("  Cache reads:  %d tokens\n", ectx.ContextStats.CacheReadTokens))
		sb.WriteString(fmt.Sprintf("  Cache writes: %d tokens\n", ectx.ContextStats.CacheWriteTokens))
		sb.WriteString(fmt.Sprintf("  Usage: %.0f%%\n", ectx.ContextStats.UsedFraction*100))
	}

	return sb.String(), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Register deep auth commands, replacing stubs.
// ──────────────────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepLoginCommand{},
		&DeepLogoutCommand{},
		&DeepUsageCommand{},
		&DeepExtraUsageCommand{},
		&ExtraUsageNonInteractiveCommand{},
		&DeepPassesCommand{},
		&UsageReportCommand{},
	)
}
