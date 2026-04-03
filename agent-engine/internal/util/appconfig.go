package util

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AppConfig is the full application configuration with layered loading.
// It merges defaults < global (~/.claude/settings.json) < project (.claude/settings.json)
// < local (.claude/settings.local.json) < environment variables < CLI flags.
type AppConfig struct {
	mu sync.RWMutex

	// Provider settings
	Provider       string  `json:"provider"`
	Model          string  `json:"model"`
	SmallModel     string  `json:"small_model"`
	APIKey         string  `json:"-"` // never serialized
	BaseURL        string  `json:"base_url,omitempty"`
	MaxTokens      int     `json:"max_tokens"`
	ThinkingBudget int     `json:"thinking_budget"`
	Temperature    float64 `json:"temperature,omitempty"`

	// Session settings
	AutoCompact bool    `json:"auto_compact"`
	AutoMode    bool    `json:"auto_mode"`
	VerboseMode bool    `json:"verbose"`
	PlanMode    bool    `json:"plan_mode"`
	MaxCostUSD  float64 `json:"max_cost_usd,omitempty"`

	// Permission settings
	PermissionMode string   `json:"permission_mode"`
	AllowedDirs    []string `json:"allowed_dirs,omitempty"`
	DeniedCommands []string `json:"denied_commands,omitempty"`

	// UI settings
	DarkMode     bool   `json:"dark_mode"`
	OutputFormat string `json:"output_format"` // "text", "json", "stream-json"

	// MCP settings
	MCPServers map[string]MCPServerConfig `json:"mcp_servers,omitempty"`

	// Memory settings
	MemoryEnabled        bool `json:"memory_enabled"`
	SessionMemoryEnabled bool `json:"session_memory_enabled"`

	// Network
	HTTPPort int    `json:"http_port"`
	Proxy    string `json:"proxy,omitempty"`

	// Internal
	WorkDir     string   `json:"-"`
	SessionID   string   `json:"-"`
	ConfigPaths []string `json:"-"` // paths that were loaded
}

// MCPServerConfig holds config for a single MCP server.
type MCPServerConfig struct {
	Command  string            `json:"command"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	URL      string            `json:"url,omitempty"`
	Type     string            `json:"type,omitempty"` // stdio, sse, http
	Disabled bool              `json:"disabled,omitempty"`
}

// DefaultAppConfig returns configuration with sensible defaults.
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Provider:             "anthropic",
		Model:                "claude-sonnet-4-5",
		SmallModel:           "claude-haiku-4-5",
		MaxTokens:            16384,
		AutoCompact:          true,
		PermissionMode:       "default",
		DarkMode:             true,
		OutputFormat:         "text",
		MemoryEnabled:        true,
		SessionMemoryEnabled: true,
		HTTPPort:             8080,
	}
}

// LoadAppConfig loads configuration from all layers.
func LoadAppConfig(workDir string) (*AppConfig, error) {
	cfg := DefaultAppConfig()
	cfg.WorkDir = workDir

	// Layer 1: Global settings (~/.claude/settings.json)
	home, _ := os.UserHomeDir()
	if home != "" {
		globalPath := filepath.Join(home, ".claude", "settings.json")
		if err := cfg.loadFromFile(globalPath); err == nil {
			cfg.ConfigPaths = append(cfg.ConfigPaths, globalPath)
		}
	}

	// Layer 2: Project settings (.claude/settings.json)
	if workDir != "" {
		projectPath := filepath.Join(workDir, ".claude", "settings.json")
		if err := cfg.loadFromFile(projectPath); err == nil {
			cfg.ConfigPaths = append(cfg.ConfigPaths, projectPath)
		}
	}

	// Layer 3: Local settings (.claude/settings.local.json)
	if workDir != "" {
		localPath := filepath.Join(workDir, ".claude", "settings.local.json")
		if err := cfg.loadFromFile(localPath); err == nil {
			cfg.ConfigPaths = append(cfg.ConfigPaths, localPath)
		}
	}

	// Layer 4: Environment variables
	cfg.loadFromEnv()

	return cfg, nil
}

// loadFromFile merges a JSON file into the config (non-zero values override).
func (c *AppConfig) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var overlay AppConfig
	if err := json.Unmarshal(data, &overlay); err != nil {
		return fmt.Errorf("parse config %q: %w", path, err)
	}
	c.merge(&overlay)
	return nil
}

// loadFromEnv reads AGENT_ENGINE_* environment variables and common aliases.
func (c *AppConfig) loadFromEnv() {
	envOr := func(keys ...string) string {
		for _, k := range keys {
			if v := os.Getenv(k); v != "" {
				return v
			}
		}
		return ""
	}

	if v := envOr("AGENT_ENGINE_PROVIDER", "LLM_PROVIDER"); v != "" {
		c.Provider = v
	}
	if v := envOr("AGENT_ENGINE_MODEL", "LLM_MODEL"); v != "" {
		c.Model = v
	}
	if v := envOr("AGENT_ENGINE_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"MINIMAX_API_KEY", "VLLM_API_KEY", "OPENROUTER_API_KEY",
		"LLM_API_KEY", "API_KEY"); v != "" {
		c.APIKey = v
	}
	if v := envOr("AGENT_ENGINE_BASE_URL", "OPENAI_BASE_URL", "VLLM_BASE_URL",
		"MINIMAX_BASE_URL", "LLM_BASE_URL", "BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := envOr("AGENT_ENGINE_PERMISSION_MODE"); v != "" {
		c.PermissionMode = v
	}
}

// merge applies non-zero values from other onto c.
func (c *AppConfig) merge(other *AppConfig) {
	if other.Provider != "" {
		c.Provider = other.Provider
	}
	if other.Model != "" {
		c.Model = other.Model
	}
	if other.SmallModel != "" {
		c.SmallModel = other.SmallModel
	}
	if other.BaseURL != "" {
		c.BaseURL = other.BaseURL
	}
	if other.MaxTokens > 0 {
		c.MaxTokens = other.MaxTokens
	}
	if other.ThinkingBudget > 0 {
		c.ThinkingBudget = other.ThinkingBudget
	}
	if other.Temperature != 0 {
		c.Temperature = other.Temperature
	}
	if other.PermissionMode != "" {
		c.PermissionMode = other.PermissionMode
	}
	if other.OutputFormat != "" {
		c.OutputFormat = other.OutputFormat
	}
	if other.HTTPPort > 0 {
		c.HTTPPort = other.HTTPPort
	}
	if other.MaxCostUSD > 0 {
		c.MaxCostUSD = other.MaxCostUSD
	}
	if other.Proxy != "" {
		c.Proxy = other.Proxy
	}
	// Booleans: only override if explicitly set in the overlay file.
	// Since JSON unmarshaling sets false for missing bool fields,
	// we can't distinguish missing from false here — this is a known
	// limitation. For booleans that need explicit false overrides,
	// use a separate mechanism (e.g. pointer or tri-state).
	if other.AutoCompact {
		c.AutoCompact = true
	}
	if other.AutoMode {
		c.AutoMode = true
	}
	if other.VerboseMode {
		c.VerboseMode = true
	}
	if other.PlanMode {
		c.PlanMode = true
	}
	if other.MemoryEnabled {
		c.MemoryEnabled = true
	}
	if other.SessionMemoryEnabled {
		c.SessionMemoryEnabled = true
	}
	if len(other.AllowedDirs) > 0 {
		c.AllowedDirs = append(c.AllowedDirs, other.AllowedDirs...)
	}
	if len(other.DeniedCommands) > 0 {
		c.DeniedCommands = append(c.DeniedCommands, other.DeniedCommands...)
	}
	if len(other.MCPServers) > 0 {
		if c.MCPServers == nil {
			c.MCPServers = make(map[string]MCPServerConfig)
		}
		for k, v := range other.MCPServers {
			c.MCPServers[k] = v
		}
	}
}

// Get returns a config value by key (dot-separated path).
func (c *AppConfig) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch strings.ToLower(key) {
	case "provider":
		return c.Provider
	case "model":
		return c.Model
	case "small_model":
		return c.SmallModel
	case "max_tokens":
		return c.MaxTokens
	case "thinking_budget":
		return c.ThinkingBudget
	case "auto_compact":
		return c.AutoCompact
	case "auto_mode":
		return c.AutoMode
	case "verbose":
		return c.VerboseMode
	case "plan_mode":
		return c.PlanMode
	case "permission_mode":
		return c.PermissionMode
	case "dark_mode":
		return c.DarkMode
	case "output_format":
		return c.OutputFormat
	case "memory_enabled":
		return c.MemoryEnabled
	case "session_memory_enabled":
		return c.SessionMemoryEnabled
	case "max_cost_usd":
		return c.MaxCostUSD
	case "http_port":
		return c.HTTPPort
	default:
		return nil
	}
}

// Set updates a config value by key. Returns error if key is unknown.
func (c *AppConfig) Set(key string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch strings.ToLower(key) {
	case "model":
		c.Model = fmt.Sprintf("%v", value)
	case "small_model":
		c.SmallModel = fmt.Sprintf("%v", value)
	case "max_tokens":
		if v, ok := value.(int); ok {
			c.MaxTokens = v
		}
	case "thinking_budget":
		if v, ok := value.(int); ok {
			c.ThinkingBudget = v
		}
	case "auto_compact":
		if v, ok := value.(bool); ok {
			c.AutoCompact = v
		}
	case "auto_mode":
		if v, ok := value.(bool); ok {
			c.AutoMode = v
		}
	case "verbose":
		if v, ok := value.(bool); ok {
			c.VerboseMode = v
		}
	case "plan_mode":
		if v, ok := value.(bool); ok {
			c.PlanMode = v
		}
	case "permission_mode":
		c.PermissionMode = fmt.Sprintf("%v", value)
	case "dark_mode":
		if v, ok := value.(bool); ok {
			c.DarkMode = v
		}
	case "output_format":
		c.OutputFormat = fmt.Sprintf("%v", value)
	case "memory_enabled":
		if v, ok := value.(bool); ok {
			c.MemoryEnabled = v
		}
	default:
		return fmt.Errorf("unknown config key: %q", key)
	}
	return nil
}

// Summary returns a human-readable config summary.
func (c *AppConfig) Summary() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("Configuration:\n")
	sb.WriteString(fmt.Sprintf("  Provider:         %s\n", c.Provider))
	sb.WriteString(fmt.Sprintf("  Model:            %s\n", c.Model))
	sb.WriteString(fmt.Sprintf("  Small model:      %s\n", c.SmallModel))
	sb.WriteString(fmt.Sprintf("  Max tokens:       %d\n", c.MaxTokens))
	sb.WriteString(fmt.Sprintf("  Thinking budget:  %d\n", c.ThinkingBudget))
	sb.WriteString(fmt.Sprintf("  Permission mode:  %s\n", c.PermissionMode))
	sb.WriteString(fmt.Sprintf("  Auto compact:     %v\n", c.AutoCompact))
	sb.WriteString(fmt.Sprintf("  Memory:           %v\n", c.MemoryEnabled))
	sb.WriteString(fmt.Sprintf("  Session memory:   %v\n", c.SessionMemoryEnabled))
	if c.MaxCostUSD > 0 {
		sb.WriteString(fmt.Sprintf("  Max cost:         $%.2f\n", c.MaxCostUSD))
	}
	if len(c.MCPServers) > 0 {
		sb.WriteString(fmt.Sprintf("  MCP servers:      %d\n", len(c.MCPServers)))
	}
	if len(c.ConfigPaths) > 0 {
		sb.WriteString("  Loaded from:\n")
		for _, p := range c.ConfigPaths {
			sb.WriteString(fmt.Sprintf("    - %s\n", p))
		}
	}
	return sb.String()
}

// SaveToProject persists the config to .claude/settings.json in the work dir.
func (c *AppConfig) SaveToProject() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.WorkDir == "" {
		return fmt.Errorf("no work directory set")
	}
	dir := filepath.Join(c.WorkDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "settings.json"), data, 0644)
}
