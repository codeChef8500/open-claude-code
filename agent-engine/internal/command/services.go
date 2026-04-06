package command

import "context"

// ──────────────────────────────────────────────────────────────────────────────
// CommandServices — aggregated service dependencies for command execution.
// Aligned with claude-code-main's ToolUseContext / LocalJSXCommandContext
// service dependencies that commands rely on during execution.
// ──────────────────────────────────────────────────────────────────────────────

// CommandServices aggregates all external service dependencies that commands
// may need during execution. Commands access this via ExecContext.Services.
type CommandServices struct {
	Session   SessionService
	Cache     CacheService
	Compact   CompactService
	Hook      HookService
	Analytics AnalyticsService
	Git       GitService
	Shell     ShellService
	Config    ConfigService
	Auth      AuthService
	MCP       MCPService
	Plugin    PluginService
	Skill     SkillService
	Task      TaskService
	AppState  AppStateService
	Browser   BrowserService
	Clipboard ClipboardService
}

// ──────────────────────────────────────────────────────────────────────────────
// Service interfaces — each maps to a subsystem that commands interact with.
// Concrete implementations live in their respective packages.
// ──────────────────────────────────────────────────────────────────────────────

// SessionService manages session lifecycle (IDs, storage, resume).
// Aligned with claude-code-main session management in clearConversation().
type SessionService interface {
	// CurrentSessionID returns the active session ID.
	CurrentSessionID() string
	// RegenerateSessionID creates a new session ID and returns it.
	RegenerateSessionID() string
	// RegenerateConversationID creates a new conversation ID and returns it.
	RegenerateConversationID() string
	// ListSessions returns metadata for all stored sessions.
	ListSessions(ctx context.Context) ([]SessionMeta, error)
	// LoadSession restores messages from a previous session.
	LoadSession(ctx context.Context, sessionID string) (interface{}, error)
	// SaveSessionMeta persists session metadata.
	SaveSessionMeta(ctx context.Context, meta SessionMeta) error
	// ResetSessionFilePointer resets the session file write cursor.
	ResetSessionFilePointer()
}

// SessionMeta is metadata for a stored session.
type SessionMeta struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	MessageCount int    `json:"message_count"`
	Model        string `json:"model"`
	CWD          string `json:"cwd"`
}

// CacheService manages clearing of various session-related caches.
// Aligned with claude-code-main clearSessionCaches() (~25 cache categories).
type CacheService interface {
	// ClearSessionCaches clears all session-related caches.
	// preservedAgentIDs lists agent IDs whose caches should be kept.
	ClearSessionCaches(preservedAgentIDs []string)
	// ClearUserContextCache clears user context caches.
	ClearUserContextCache()
	// ClearSystemContextCache clears system context caches.
	ClearSystemContextCache()
	// ClearGitStatusCache clears cached git status.
	ClearGitStatusCache()
	// ClearCommandCache clears memoized command lists.
	ClearCommandCache()
	// ClearSkillCache clears cached skill definitions.
	ClearSkillCache()
	// ClearDynamicSkills clears dynamically discovered skills.
	ClearDynamicSkills()
	// ClearPostCompactCleanup runs post-compaction cache cleanup.
	ClearPostCompactCleanup()
	// ClearAll clears everything.
	ClearAll()
}

// CompactService handles conversation compaction.
// Aligned with claude-code-main services/compact/.
type CompactService interface {
	// CompactConversation performs full conversation compaction.
	CompactConversation(ctx context.Context, opts CompactOptions) (*CompactResult, error)
	// MicrocompactMessages removes redundant tool output to save tokens.
	MicrocompactMessages(ctx context.Context, messages interface{}) (interface{}, error)
	// TrySessionMemoryCompaction attempts session-memory based compaction.
	TrySessionMemoryCompaction(ctx context.Context, opts CompactOptions) (*CompactResult, bool, error)
	// ReactiveCompact performs reactive compaction when prompt is too long.
	ReactiveCompact(ctx context.Context, opts CompactOptions) (*CompactResult, error)
}

// CompactOptions configures a compaction request.
type CompactOptions struct {
	Messages          interface{}
	SystemPrompt      string
	CustomInstruction string
	Model             string
	MaxTokens         int
	PreserveLastN     int
	CompactBoundaryID string
}

// CompactResult is the outcome of a compaction operation.
type CompactResult struct {
	Messages     interface{} `json:"messages"`
	Summary      string      `json:"summary"`
	TokensBefore int         `json:"tokens_before"`
	TokensAfter  int         `json:"tokens_after"`
	Strategy     string      `json:"strategy"`
}

// HookService executes lifecycle hooks.
// Aligned with claude-code-main hooks system.
type HookService interface {
	// ExecuteSessionStartHooks runs hooks at session start.
	ExecuteSessionStartHooks(ctx context.Context) error
	// ExecuteSessionEndHooks runs hooks at session end with timeout.
	ExecuteSessionEndHooks(ctx context.Context) error
	// ExecutePreToolUseHooks runs hooks before a tool execution.
	ExecutePreToolUseHooks(ctx context.Context, toolName string, input interface{}) error
	// ExecutePostToolUseHooks runs hooks after a tool execution.
	ExecutePostToolUseHooks(ctx context.Context, toolName string, output interface{}) error
}

// AnalyticsService logs telemetry events.
type AnalyticsService interface {
	// Track logs an analytics event.
	Track(event string, properties map[string]interface{})
	// SendCacheEvictionHint sends a cache eviction hint event.
	SendCacheEvictionHint()
}

// GitService provides git operations.
// Aligned with claude-code-main git utility calls in commit/review/diff commands.
type GitService interface {
	// Status returns `git status` output.
	Status(ctx context.Context, dir string) (string, error)
	// Diff returns `git diff` output.
	Diff(ctx context.Context, dir string, args ...string) (string, error)
	// Log returns `git log` output.
	Log(ctx context.Context, dir string, args ...string) (string, error)
	// Show returns `git show` output.
	Show(ctx context.Context, dir string, args ...string) (string, error)
	// Add stages files.
	Add(ctx context.Context, dir string, paths ...string) error
	// Commit creates a commit.
	Commit(ctx context.Context, dir string, message string) error
	// CurrentBranch returns the current branch name.
	CurrentBranch(ctx context.Context, dir string) (string, error)
	// ListCheckpoints returns commit hashes for rewind.
	ListCheckpoints(ctx context.Context, dir string) ([]GitCheckpoint, error)
	// RevertTo reverts the working tree to a checkpoint.
	RevertTo(ctx context.Context, dir string, commitHash string) error
}

// GitCheckpoint is a rewind point.
type GitCheckpoint struct {
	Hash      string `json:"hash"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	Turn      int    `json:"turn"`
}

// ShellService executes shell commands.
// Aligned with claude-code-main utils/promptShellExecution.ts.
type ShellService interface {
	// Execute runs a shell command and returns stdout.
	Execute(ctx context.Context, dir string, command string) (string, error)
	// ExecuteWithTimeout runs a command with a timeout.
	ExecuteWithTimeout(ctx context.Context, dir string, command string, timeoutSec int) (string, error)
}

// ConfigService reads and writes configuration.
// Aligned with claude-code-main config module.
type ConfigService interface {
	// Get returns a config value by key.
	Get(key string) (interface{}, bool)
	// Set sets a config value.
	Set(key string, value interface{}) error
	// List returns all config key-value pairs with their source.
	List() []ConfigEntry
	// GetSource returns the source of a config key (project/user/default).
	GetSource(key string) string
	// ProjectPath returns the project config file path.
	ProjectPath() string
	// UserPath returns the user config file path.
	UserPath() string
}

// ConfigEntry is a single configuration item with source info.
type ConfigEntry struct {
	Key    string      `json:"key"`
	Value  interface{} `json:"value"`
	Source string      `json:"source"` // "project", "user", "default"
}

// AuthService handles authentication state.
// Aligned with claude-code-main login/logout commands.
type AuthService interface {
	// IsAuthenticated returns true if the user is logged in.
	IsAuthenticated() bool
	// IsClaudeAISubscriber returns true for claude.ai subscribers.
	IsClaudeAISubscriber() bool
	// IsUsing3PServices returns true for 3P services (Bedrock/Vertex).
	IsUsing3PServices() bool
	// IsFirstPartyAnthropicBaseUrl returns true for 1P Anthropic API.
	IsFirstPartyAnthropicBaseUrl() bool
	// Login initiates OAuth flow.
	Login(ctx context.Context) error
	// Logout clears authentication state.
	Logout(ctx context.Context) error
	// GetAPIKey returns the current API key.
	GetAPIKey() string
	// UserType returns "ant" or empty string.
	UserType() string
}

// MCPService manages MCP server connections.
// Aligned with claude-code-main MCP management in /mcp command.
type MCPService interface {
	// ListServers returns all configured MCP servers.
	ListServers() []MCPServerInfo
	// AddServer adds a new MCP server configuration.
	AddServer(name string, config map[string]interface{}) error
	// RemoveServer removes an MCP server.
	RemoveServer(name string) error
	// RestartServer restarts a specific MCP server.
	RestartServer(ctx context.Context, name string) error
	// GetServerTools returns tools provided by a server.
	GetServerTools(name string) []interface{}
	// GetSlashCommands returns slash commands provided by a server.
	GetSlashCommands(serverName string) []MCPSlashCommand
	// GetSlashCommandContent returns the prompt content for an MCP slash command.
	GetSlashCommandContent(serverName, commandName string) (string, error)
}

// MCPSlashCommand describes a slash command provided by an MCP server.
type MCPSlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MCPServerInfo describes an MCP server.
type MCPServerInfo struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // "connected", "disconnected", "error"
	Transport string `json:"transport"`
	ToolCount int    `json:"tool_count"`
	Error     string `json:"error,omitempty"`
}

// PluginService manages plugin lifecycle.
// Aligned with claude-code-main plugin loading.
type PluginService interface {
	// ListPlugins returns installed plugins.
	ListPlugins() []PluginMeta
	// InstallPlugin installs a plugin from a repository.
	InstallPlugin(ctx context.Context, repo string) error
	// UninstallPlugin removes a plugin.
	UninstallPlugin(name string) error
	// EnablePlugin enables a plugin.
	EnablePlugin(name string) error
	// DisablePlugin disables a plugin.
	DisablePlugin(name string) error
	// ReloadAll reloads all plugins and returns newly registered commands.
	ReloadAll() ([]Command, error)
	// GetPluginCommands returns commands registered by plugins.
	GetPluginCommands() []Command
	// GetPluginSkills returns skills registered by plugins.
	GetPluginSkills() []Command
}

// PluginMeta describes an installed plugin.
type PluginMeta struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Enabled    bool   `json:"enabled"`
}

// SkillService handles skill discovery and loading.
// Aligned with claude-code-main skill loading pipeline.
type SkillService interface {
	// LoadSkillDirs loads skills from project and user directories.
	LoadSkillDirs(workDir string) []Command
	// GetBundledSkills returns bundled (embedded) skills.
	GetBundledSkills() []Command
	// GetDynamicSkills returns dynamically discovered skills.
	GetDynamicSkills() []Command
	// ClearDynamicSkills clears the dynamic skills cache.
	ClearDynamicSkills()
}

// TaskService manages background tasks/agents.
// Aligned with claude-code-main task management in clear/resume commands.
type TaskService interface {
	// ListTasks returns active background tasks.
	ListTasks() []TaskInfo
	// GetPreservedAgentIDs returns agent IDs that should survive /clear.
	GetPreservedAgentIDs() []string
	// RelinkTaskOutputs re-creates symlinks for preserved tasks after /clear.
	RelinkTaskOutputs(preservedIDs []string) error
}

// TaskInfo describes a background task.
type TaskInfo struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	Status    string `json:"status"` // "running", "completed", "failed"
	Title     string `json:"title"`
	CreatedAt int64  `json:"created_at"`
}

// AppStateService provides access to mutable application state.
// Aligned with claude-code-main AppState / getAppState().
type AppStateService interface {
	// GetWorkDir returns the original working directory.
	GetOriginalWorkDir() string
	// ResetCWD resets the working directory to original.
	ResetCWD()
	// ClearFileHistory clears the file edit history.
	ClearFileHistory()
	// ClearAttribution clears attribution state.
	ClearAttribution()
	// ClearMCPState resets dynamic MCP configuration.
	ClearMCPState()
	// ClearPlanSlug clears the plan slug cache.
	ClearPlanSlug()
	// ClearSessionMetadata clears session metadata.
	ClearSessionMetadata()
	// SaveModeState persists current mode (plan/fast/effort).
	SaveModeState()
	// SaveWorktreeState persists worktree configuration.
	SaveWorktreeState()
	// IsDemo returns true if running in demo mode.
	IsDemo() bool
}

// BrowserService opens URLs in the default browser.
type BrowserService interface {
	// Open opens a URL in the default browser.
	Open(url string) error
}

// ClipboardService provides clipboard access.
type ClipboardService interface {
	// Copy copies text to the system clipboard.
	Copy(text string) error
	// Read reads text from the system clipboard.
	Read() (string, error)
}

// ──────────────────────────────────────────────────────────────────────────────
// Nil-safe service accessors
// ──────────────────────────────────────────────────────────────────────────────

// NilServices returns a zero-value CommandServices (all nil).
// Commands should check for nil services before calling methods.
func NilServices() *CommandServices { return &CommandServices{} }
