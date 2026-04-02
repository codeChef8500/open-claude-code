package command

import "context"

// CommandType distinguishes built-in local commands from prompt-injection commands.
type CommandType string

const (
	CommandTypeLocal  CommandType = "local"
	CommandTypePrompt CommandType = "prompt"
	CommandTypeMeta   CommandType = "meta" // meta commands that modify engine state (e.g. /model, /config)
)

// CommandSource indicates who registered the command.
type CommandSource string

const (
	CommandSourceBuiltin CommandSource = "builtin"
	CommandSourcePlugin  CommandSource = "plugin"
	CommandSourceUser    CommandSource = "user"
	CommandSourceMCP     CommandSource = "mcp"
)

// Command defines the contract for all slash commands.
type Command interface {
	// Name is the slash-command identifier (without the leading "/").
	Name() string
	// Aliases returns alternate names for the command (e.g. "q" for "quit").
	Aliases() []string
	// Description is a short human-readable description.
	Description() string
	// Type distinguishes local (imperative) from prompt (injected) commands.
	Type() CommandType
	// Source returns who registered this command.
	Source() CommandSource
	// IsEnabled reports whether this command should appear in the registry.
	IsEnabled(ctx *ExecContext) bool
	// GetCompletions returns tab-completion candidates for the given partial args.
	GetCompletions(args string, ectx *ExecContext) []Completion
}

// Completion is a single tab-completion candidate.
type Completion struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	InsertText  string `json:"insert_text,omitempty"`
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

// MetaCommand modifies engine state and optionally returns display text.
type MetaCommand interface {
	Command
	// Execute runs the meta command, modifying state and returning
	// an optional status message for display.
	Execute(ctx context.Context, args []string, ectx *ExecContext) (string, error)
}

// CommandRegistry is the interface for looking up and listing commands.
type CommandRegistry interface {
	// Get returns the command with the given name or alias, or nil.
	Get(name string) Command
	// List returns all registered commands.
	List() []Command
	// Register adds a command to the registry.
	Register(cmd Command)
	// IsSlashCommand reports whether the input string starts with a known
	// slash command.
	IsSlashCommand(input string) bool
}

// BaseCommand provides default implementations for the expanded Command
// interface methods. Embed it in command structs to avoid boilerplate.
type BaseCommand struct{}

func (b *BaseCommand) Aliases() []string                                    { return nil }
func (b *BaseCommand) Source() CommandSource                                { return CommandSourceBuiltin }
func (b *BaseCommand) GetCompletions(_ string, _ *ExecContext) []Completion { return nil }

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

	// PermissionMode is the current permission mode.
	PermissionMode string
	// PlanModeActive is true when the session is in plan-only mode.
	PlanModeActive bool
	// FastMode is true when fast mode is active.
	FastMode bool
	// TurnCount is the number of conversation turns so far.
	TurnCount int
	// TotalTokens is the cumulative token count for the session.
	TotalTokens int
	// ActiveMCPServers lists connected MCP server names.
	ActiveMCPServers []string
}
