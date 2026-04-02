package command

import "context"

// CommandType distinguishes built-in local commands from prompt-injection commands.
type CommandType string

const (
	CommandTypeLocal  CommandType = "local"
	CommandTypePrompt CommandType = "prompt"
)

// Command defines the contract for all slash commands.
type Command interface {
	// Name is the slash-command identifier (without the leading "/").
	Name() string
	// Description is a short human-readable description.
	Description() string
	// Type distinguishes local (imperative) from prompt (injected) commands.
	Type() CommandType
	// IsEnabled reports whether this command should appear in the registry.
	IsEnabled(ctx *ExecContext) bool
}

// LocalCommand can execute code directly.
type LocalCommand interface {
	Command
	// Execute runs the command and returns a result or error.
	Execute(ctx context.Context, args []string, ectx *ExecContext) (string, error)
}

// PromptCommand injects content into the conversation.
type PromptCommand interface {
	Command
	// PromptContent returns the text to inject as a user message.
	PromptContent(args []string, ectx *ExecContext) (string, error)
}

// ExecContext holds the environment available to a command during execution.
type ExecContext struct {
	WorkDir     string
	SessionID   string
	AutoMode    bool
	Verbose     bool
	Model       string
	CostUSD     float64
	PrintOutput func(string)
	// ContextStats is optionally populated by the engine to expose token
	// budget information to the /context command.
	ContextStats *ContextStats
}
