package plugin

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ── Plugin Manifest (plugin.json) ────────────────────────────────────────────

// PluginManifest is the Go representation of a plugin.json file.
// Mirrors claude-code-main's PluginManifestSchema.
type PluginManifest struct {
	// Metadata
	Name        string        `json:"name"`
	Version     string        `json:"version,omitempty"`
	Description string        `json:"description,omitempty"`
	Author      *PluginAuthor `json:"author,omitempty"`
	Homepage    string        `json:"homepage,omitempty"`
	Repository  string        `json:"repository,omitempty"`
	License     string        `json:"license,omitempty"`
	Keywords    []string      `json:"keywords,omitempty"`

	// Commands: paths or inline definitions
	Commands json.RawMessage `json:"commands,omitempty"`
	// Agents: paths to agent markdown files
	Agents json.RawMessage `json:"agents,omitempty"`
	// Skills: paths to skill directories
	Skills json.RawMessage `json:"skills,omitempty"`
	// OutputStyles: paths to output style files/dirs
	OutputStyles json.RawMessage `json:"outputStyles,omitempty"`
	// Hooks: inline hooks config or path to hooks JSON
	Hooks json.RawMessage `json:"hooks,omitempty"`

	// MCP servers: inline configs, paths, or MCPB files
	MCPServers json.RawMessage `json:"mcpServers,omitempty"`
	// LSP servers: inline configs or paths
	LSPServers json.RawMessage `json:"lspServers,omitempty"`

	// User configuration options
	UserConfig map[string]*UserConfigOption `json:"userConfig,omitempty"`
	// Channels for assistant mode
	Channels []ChannelConfig `json:"channels,omitempty"`
	// Settings to merge when plugin is enabled
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// PluginAuthor describes the plugin creator.
type PluginAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// UserConfigOption defines a single user-configurable value.
type UserConfigOption struct {
	Type        string      `json:"type"` // "string", "number", "boolean", "directory", "file"
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Required    bool        `json:"required,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Multiple    bool        `json:"multiple,omitempty"`
	Sensitive   bool        `json:"sensitive,omitempty"`
	Min         *float64    `json:"min,omitempty"`
	Max         *float64    `json:"max,omitempty"`
}

// ChannelConfig declares an MCP server as a message channel.
type ChannelConfig struct {
	Server      string                      `json:"server"`
	DisplayName string                      `json:"displayName,omitempty"`
	UserConfig  map[string]*UserConfigOption `json:"userConfig,omitempty"`
}

// CommandMetadata provides rich metadata for a plugin command.
type CommandMetadata struct {
	Source       string   `json:"source,omitempty"`
	Content      string   `json:"content,omitempty"`
	Description  string   `json:"description,omitempty"`
	ArgumentHint string   `json:"argumentHint,omitempty"`
	Model        string   `json:"model,omitempty"`
	AllowedTools []string `json:"allowedTools,omitempty"`
}

// LspServerConfig describes an LSP server.
type LspServerConfig struct {
	Command               string                 `json:"command"`
	Args                  []string               `json:"args,omitempty"`
	ExtensionToLanguage   map[string]string       `json:"extensionToLanguage"`
	Transport             string                 `json:"transport,omitempty"` // "stdio" or "socket"
	Env                   map[string]string       `json:"env,omitempty"`
	InitializationOptions interface{}            `json:"initializationOptions,omitempty"`
	Settings              interface{}            `json:"settings,omitempty"`
	WorkspaceFolder       string                 `json:"workspaceFolder,omitempty"`
	StartupTimeout        int                    `json:"startupTimeout,omitempty"`
	ShutdownTimeout       int                    `json:"shutdownTimeout,omitempty"`
	RestartOnCrash        bool                   `json:"restartOnCrash,omitempty"`
	MaxRestarts           int                    `json:"maxRestarts,omitempty"`
}

// ── Loaded Plugin ────────────────────────────────────────────────────────────

// PluginSource indicates where a plugin was loaded from.
type PluginSource string

const (
	SourceMarketplace PluginSource = "marketplace"
	SourceInline      PluginSource = "inline"      // --plugin-dir CLI flag
	SourceBuiltin     PluginSource = "builtin"
	SourceExternal    PluginSource = "external"     // binary plugins (go-plugin)
)

// LoadedPlugin is a manifest-based plugin that has been loaded and validated.
type LoadedPlugin struct {
	// Name is the plugin name (from manifest).
	Name string `json:"name"`
	// Manifest is the parsed plugin.json.
	Manifest *PluginManifest `json:"manifest,omitempty"`
	// Path is the absolute path to the plugin directory.
	Path string `json:"path"`
	// Source indicates where the plugin came from.
	Source PluginSource `json:"source"`
	// Repository is the source repository URL (marketplace plugins).
	Repository string `json:"repository,omitempty"`
	// Enabled indicates whether the user has enabled this plugin.
	Enabled bool `json:"enabled"`
	// IsBuiltin indicates this is a builtin plugin.
	IsBuiltin bool `json:"is_builtin,omitempty"`

	// Resolved paths (populated at load time)
	CommandsPath string `json:"commands_path,omitempty"`
	SkillsPath   string `json:"skills_path,omitempty"`
	AgentsPath   string `json:"agents_path,omitempty"`

	// Resolved configurations
	HooksConfig map[string]interface{} `json:"hooks_config,omitempty"`
	MCPConfigs  map[string]interface{} `json:"mcp_configs,omitempty"`
	LSPConfigs  map[string]*LspServerConfig `json:"lsp_configs,omitempty"`

	// User config values (populated after user interaction)
	UserConfigValues map[string]interface{} `json:"user_config_values,omitempty"`

	// Merged settings from manifest
	MergedSettings map[string]interface{} `json:"merged_settings,omitempty"`
}

// ── Builtin Plugin Definition ────────────────────────────────────────────────

// BuiltinPluginDefinition describes a builtin plugin that ships with the engine.
// Matching claude-code-main's BuiltinPluginDefinition.
type BuiltinPluginDefinition struct {
	// Name is the unique plugin name.
	Name string
	// Description is a user-facing description.
	Description string
	// Version is the plugin version.
	Version string
	// Skills are the bundled skill definitions.
	Skills []BuiltinPluginSkill
	// Hooks are the hook handlers keyed by HookType.
	Hooks map[HookType][]HookHandler
	// IsAvailable returns whether this plugin can be used in the current environment.
	// Nil means always available.
	IsAvailable func() bool
	// DefaultEnabled is whether this plugin is enabled by default.
	DefaultEnabled bool
}

// BuiltinPluginSkill describes a skill provided by a builtin plugin.
type BuiltinPluginSkill struct {
	Name         string
	Description  string
	WhenToUse    string
	AllowedTools []string
	Prompt       string
}

// ── Plugin Load Result ──────────────────────────────────────────────────────

// PluginLoadResult aggregates the result of loading all plugins.
type PluginLoadResult struct {
	Enabled  []*LoadedPlugin `json:"enabled"`
	Disabled []*LoadedPlugin `json:"disabled"`
	Errors   []PluginError   `json:"errors,omitempty"`
}

// PluginError describes an error encountered while loading a plugin.
type PluginError struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e PluginError) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("plugin %s: %s", e.Name, e.Message)
	}
	return e.Message
}

// ── Manifest Validation ─────────────────────────────────────────────────────

var (
	pluginNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
)

// ValidateManifest performs structural validation of a PluginManifest.
func ValidateManifest(m *PluginManifest) error {
	if m.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	if strings.Contains(m.Name, " ") {
		return fmt.Errorf("plugin name %q cannot contain spaces; use kebab-case", m.Name)
	}
	if !pluginNameRe.MatchString(m.Name) {
		return fmt.Errorf("plugin name %q is not a valid kebab-case identifier", m.Name)
	}
	// Validate user config keys.
	if m.UserConfig != nil {
		keyRe := regexp.MustCompile(`^[A-Za-z_]\w*$`)
		for k, opt := range m.UserConfig {
			if !keyRe.MatchString(k) {
				return fmt.Errorf("user config key %q is not a valid identifier", k)
			}
			if opt.Title == "" {
				return fmt.Errorf("user config option %q must have a title", k)
			}
			switch opt.Type {
			case "string", "number", "boolean", "directory", "file":
			default:
				return fmt.Errorf("user config option %q has invalid type %q", k, opt.Type)
			}
		}
	}
	return nil
}

// ── Marketplace Name Validation ─────────────────────────────────────────────

var allowedOfficialNames = map[string]struct{}{
	"claude-code-marketplace": {},
	"claude-code-plugins":     {},
	"claude-plugins-official":  {},
	"anthropic-marketplace":    {},
	"anthropic-plugins":        {},
	"agent-skills":             {},
	"life-sciences":            {},
	"knowledge-work-plugins":   {},
}

var blockedOfficialPattern = regexp.MustCompile(
	`(?i)(?:official[^a-z0-9]*(anthropic|claude)|(?:anthropic|claude)[^a-z0-9]*official|^(?:anthropic|claude)[^a-z0-9]*(marketplace|plugins|official))`,
)

// IsBlockedOfficialName returns true if the name impersonates an official marketplace.
func IsBlockedOfficialName(name string) bool {
	lower := strings.ToLower(name)
	if _, ok := allowedOfficialNames[lower]; ok {
		return false
	}
	// Block non-ASCII (homograph prevention).
	for _, r := range name {
		if r < 0x20 || r > 0x7E {
			return true
		}
	}
	return blockedOfficialPattern.MatchString(name)
}

// ValidateMarketplaceName checks a marketplace name for validity.
func ValidateMarketplaceName(name string) error {
	if name == "" {
		return fmt.Errorf("marketplace must have a name")
	}
	if strings.Contains(name, " ") {
		return fmt.Errorf("marketplace name cannot contain spaces; use kebab-case")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") ||
		strings.Contains(name, "..") || name == "." {
		return fmt.Errorf("marketplace name cannot contain path separators or '..'")
	}
	if IsBlockedOfficialName(name) {
		return fmt.Errorf("marketplace name %q impersonates an official marketplace", name)
	}
	lower := strings.ToLower(name)
	if lower == "inline" {
		return fmt.Errorf("marketplace name %q is reserved for --plugin-dir session plugins", name)
	}
	if lower == "builtin" {
		return fmt.Errorf("marketplace name %q is reserved for built-in plugins", name)
	}
	return nil
}
