package command

import "context"

// ──────────────────────────────────────────────────────────────────────────────
// Command types
// ──────────────────────────────────────────────────────────────────────────────

// CommandType distinguishes built-in local commands from prompt-injection commands.
type CommandType string

const (
	CommandTypeLocal       CommandType = "local"
	CommandTypePrompt      CommandType = "prompt"
	CommandTypeMeta        CommandType = "meta"        // meta commands that modify engine state (e.g. /model, /config)
	CommandTypeInteractive CommandType = "interactive" // interactive commands returning structured UI data (Go equivalent of local-jsx)
)

// CommandSource indicates who registered the command.
type CommandSource string

const (
	CommandSourceBuiltin CommandSource = "builtin"
	CommandSourcePlugin  CommandSource = "plugin"
	CommandSourceUser    CommandSource = "user"
	CommandSourceMCP     CommandSource = "mcp"
	CommandSourceSDK     CommandSource = "sdk"
	CommandSourceBundled CommandSource = "bundled"
	CommandSourceCustom  CommandSource = "custom"
)

// CommandAvailability restricts where a command can be used.
type CommandAvailability string

const (
	AvailabilityClaudeAI CommandAvailability = "claude-ai"
	AvailabilityConsole  CommandAvailability = "console"
)

// CommandLoadedFrom records how the command was loaded.
type CommandLoadedFrom string

const (
	LoadedFromCommands CommandLoadedFrom = "commands"
	LoadedFromSkills   CommandLoadedFrom = "skills"
	LoadedFromPlugin   CommandLoadedFrom = "plugin"
	LoadedFromManaged  CommandLoadedFrom = "managed"
	LoadedFromBundled  CommandLoadedFrom = "bundled"
	LoadedFromMCP      CommandLoadedFrom = "mcp"
)

// CommandKind is an optional classification (e.g. "workflow").
type CommandKind string

const (
	CommandKindDefault  CommandKind = ""
	CommandKindWorkflow CommandKind = "workflow"
)

// ──────────────────────────────────────────────────────────────────────────────
// Core interfaces
// ──────────────────────────────────────────────────────────────────────────────

// Command defines the contract for all slash commands.
// Aligned with claude-code-main types/command.ts CommandBase.
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

	// ── Extended metadata (aligned with CommandBase from claude-code-main) ──

	// Availability restricts where this command can be used.
	Availability() []CommandAvailability
	// IsHidden reports whether this command is hidden from /help output.
	IsHidden() bool
	// ArgumentHint returns a hint string for args (e.g. "[on|off]").
	ArgumentHint() string
	// WhenToUse returns a description of when the model should invoke this command.
	WhenToUse() string
	// Version returns the command version string.
	Version() string
	// DisableModelInvocation prevents the model from using this as a tool.
	DisableModelInvocation() bool
	// UserInvocable reports whether users can invoke this command directly.
	UserInvocable() bool
	// LoadedFrom records how this command was loaded.
	LoadedFrom() CommandLoadedFrom
	// Kind returns the command classification (e.g. "workflow").
	Kind() CommandKind
	// IsImmediate reports whether this command executes even during streaming.
	IsImmediate() bool
	// IsSensitive reports whether command arguments should be redacted.
	IsSensitive() bool
	// UserFacingName returns the display name (may differ from Name).
	UserFacingName() string
	// IsMCP reports whether this command was loaded from an MCP server.
	IsMCP() bool
}

// Completion is a single tab-completion candidate.
type Completion struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	InsertText  string `json:"insert_text,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Concrete command interfaces
// ──────────────────────────────────────────────────────────────────────────────

// LocalCommand can execute code directly.
type LocalCommand interface {
	Command
	// Execute runs the command and returns a result or error.
	Execute(ctx context.Context, args []string, ectx *ExecContext) (string, error)
}

// PromptCommand injects content into the conversation.
// Aligned with claude-code-main types/command.ts PromptCommand.
type PromptCommand interface {
	Command
	// PromptContent returns the text to inject as a user message.
	PromptContent(args []string, ectx *ExecContext) (string, error)
	// PromptMeta returns optional metadata for prompt commands.
	PromptMeta() *PromptCommandMeta
}

// MetaCommand modifies engine state and optionally returns display text.
type MetaCommand interface {
	Command
	// Execute runs the meta command, modifying state and returning
	// an optional status message for display.
	Execute(ctx context.Context, args []string, ectx *ExecContext) (string, error)
}

// InteractiveCommand returns structured data for TUI rendering.
// This is the Go equivalent of claude-code-main's LocalJSXCommand.
type InteractiveCommand interface {
	Command
	// ExecuteInteractive runs the command and returns structured result data.
	ExecuteInteractive(ctx context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error)
}

// ──────────────────────────────────────────────────────────────────────────────
// Result types
// ──────────────────────────────────────────────────────────────────────────────

// CommandResultType classifies the result of a command execution.
type CommandResultType string

const (
	ResultTypeText    CommandResultType = "text"
	ResultTypeCompact CommandResultType = "compact"
	ResultTypeSkip    CommandResultType = "skip"
)

// CommandResult is a typed result from a LocalCommand execution.
// Aligned with claude-code-main LocalCommandResult.
type CommandResult struct {
	Type        CommandResultType `json:"type"`
	Value       string            `json:"value,omitempty"`
	DisplayText string            `json:"display_text,omitempty"`
}

// InteractiveResult carries structured data for TUI rendering.
// Go equivalent of the React.ReactNode returned by LocalJSXCommand.
type InteractiveResult struct {
	// Component identifies which TUI component to render.
	Component string `json:"component"`
	// Data is the structured payload for the component.
	Data interface{} `json:"data,omitempty"`
	// HideInput hides the input prompt while this component is active.
	HideInput bool `json:"hide_input,omitempty"`
	// OnDone is called when the interactive component completes.
	OnDone func(result interface{}) `json:"-"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Prompt command metadata
// ──────────────────────────────────────────────────────────────────────────────

// PromptCommandMeta carries optional metadata for prompt commands.
// Aligned with claude-code-main PromptCommand fields.
type PromptCommandMeta struct {
	// ProgressMessage is shown while the prompt is being processed.
	ProgressMessage string `json:"progress_message,omitempty"`
	// ContentLength is the estimated length of prompt content in chars.
	ContentLength int `json:"content_length,omitempty"`
	// AllowedTools restricts which tools can be used during prompt execution.
	AllowedTools []string `json:"allowed_tools,omitempty"`
	// Model overrides the model for this prompt command.
	Model string `json:"model,omitempty"`
	// Effort controls the effort level ("low", "medium", "high", "max").
	Effort string `json:"effort,omitempty"`
	// Hooks to register when this skill is invoked.
	Hooks *HooksSettings `json:"hooks,omitempty"`
	// SkillRoot is the base directory for skill resources.
	SkillRoot string `json:"skill_root,omitempty"`
	// ExecContext is "inline" (default) or "fork" (run as sub-agent).
	ExecContext string `json:"exec_context,omitempty"`
	// Agent is the sub-agent type to use for "fork" context.
	Agent string `json:"agent,omitempty"`
	// Paths are glob patterns for conditional skill activation.
	Paths []string `json:"paths,omitempty"`
	// PluginInfo carries metadata when the command originates from a plugin.
	PluginInfo *PluginInfo `json:"plugin_info,omitempty"`
	// DisableNonInteractive prevents use in non-interactive sessions.
	DisableNonInteractive bool `json:"disable_non_interactive,omitempty"`
	// ArgNames lists expected argument names for the prompt.
	ArgNames []string `json:"arg_names,omitempty"`
}

// PluginInfo carries metadata when a command comes from a plugin.
type PluginInfo struct {
	Repository string      `json:"repository"`
	Manifest   interface{} `json:"manifest,omitempty"`
}

// HooksSettings configures hooks to run at various lifecycle points.
type HooksSettings struct {
	PreToolUse  []HookEntry `json:"pre_tool_use,omitempty"`
	PostToolUse []HookEntry `json:"post_tool_use,omitempty"`
	PreSubmit   []HookEntry `json:"pre_submit,omitempty"`
	PostSubmit  []HookEntry `json:"post_submit,omitempty"`
	Stop        []HookEntry `json:"stop,omitempty"`
}

// HookEntry is a single hook command with optional matcher and timeout.
type HookEntry struct {
	Command string `json:"command"`
	Matcher string `json:"matcher,omitempty"` // tool name pattern
	Timeout int    `json:"timeout,omitempty"` // seconds
}

// ──────────────────────────────────────────────────────────────────────────────
// Registry interface
// ──────────────────────────────────────────────────────────────────────────────

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

// ──────────────────────────────────────────────────────────────────────────────
// BaseCommand — default implementations
// ──────────────────────────────────────────────────────────────────────────────

// BaseCommand provides default implementations for the expanded Command
// interface methods. Embed it in command structs to avoid boilerplate.
type BaseCommand struct{}

func (b *BaseCommand) Aliases() []string                                    { return nil }
func (b *BaseCommand) Source() CommandSource                                { return CommandSourceBuiltin }
func (b *BaseCommand) GetCompletions(_ string, _ *ExecContext) []Completion { return nil }
func (b *BaseCommand) Availability() []CommandAvailability                  { return nil }
func (b *BaseCommand) IsHidden() bool                                       { return false }
func (b *BaseCommand) ArgumentHint() string                                 { return "" }
func (b *BaseCommand) WhenToUse() string                                    { return "" }
func (b *BaseCommand) Version() string                                      { return "" }
func (b *BaseCommand) DisableModelInvocation() bool                         { return false }
func (b *BaseCommand) UserInvocable() bool                                  { return true }
func (b *BaseCommand) LoadedFrom() CommandLoadedFrom                        { return LoadedFromCommands }
func (b *BaseCommand) Kind() CommandKind                                    { return CommandKindDefault }
func (b *BaseCommand) IsImmediate() bool                                    { return false }
func (b *BaseCommand) IsSensitive() bool                                    { return false }
func (b *BaseCommand) UserFacingName() string                               { return "" }
func (b *BaseCommand) IsMCP() bool                                          { return false }

// BasePromptCommand provides a default PromptMeta() returning nil.
// Embed in prompt command structs that don't need custom metadata.
type BasePromptCommand struct {
	BaseCommand
}

func (b *BasePromptCommand) PromptMeta() *PromptCommandMeta { return nil }

// ──────────────────────────────────────────────────────────────────────────────
// ExecContext
// ──────────────────────────────────────────────────────────────────────────────

// ExecContext holds the environment available to a command during execution.
// Aligned with both ExecContext (Go) and LocalJSXCommandContext (TS).
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

	// AddWorkingDir is a callback to add a directory to the session's
	// permitted working directories (used by /add-dir).
	AddWorkingDir func(dir string) error

	// ── Extended fields (aligned with LocalJSXCommandContext from TS) ────

	// EffortLevel is the current effort setting ("low","medium","high","max","auto").
	EffortLevel string
	// Theme is the current terminal theme name.
	Theme string
	// Messages provides access to the current conversation messages.
	// The concrete type is interface{} to avoid import cycles with engine.
	Messages interface{}
	// SetMessages allows commands to update the conversation messages.
	SetMessages func(updater func(prev interface{}) interface{})
	// Tools lists the currently available tools.
	Tools []interface{}
	// Commands lists all registered commands (for /help and introspection).
	Commands []Command
	// MCPClients lists active MCP client instances.
	MCPClients []interface{}
	// APIKey is the current API key (if available, for auth commands).
	APIKey string
	// DynamicMcpConfig holds dynamic MCP configuration.
	DynamicMcpConfig map[string]interface{}
	// OnChangeAPIKey callback for API key rotation.
	OnChangeAPIKey func()
	// OnChangeDynamicMcpConfig callback for MCP config changes.
	OnChangeDynamicMcpConfig func(config map[string]interface{})
	// AbortController provides cancellation for long-running commands.
	AbortController context.Context
	// EngineRef provides access to the engine instance (opaque to avoid cycles).
	EngineRef interface{}
	// RegistryRef provides access to the command registry.
	RegistryRef *Registry
	// Services provides access to aggregated service dependencies.
	// Commands should nil-check before use.
	Services *CommandServices
}
