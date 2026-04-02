package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ServerConfig holds the configuration for a single MCP server connection.
type ServerConfig struct {
	// Name is the logical identifier for this server (used in tool namespacing).
	Name string `json:"name" yaml:"name"`
	// Transport is "stdio" or "sse".
	Transport string `json:"transport" yaml:"transport"`

	// Stdio transport fields.
	Command string   `json:"command,omitempty" yaml:"command,omitempty"`
	Args    []string `json:"args,omitempty"    yaml:"args,omitempty"`
	Env     []string `json:"env,omitempty"     yaml:"env,omitempty"`

	// SSE transport fields.
	URL     string            `json:"url,omitempty"     yaml:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`

	// Disabled excludes this server from auto-start.
	Disabled bool `json:"disabled,omitempty" yaml:"disabled,omitempty"`
}

// Validate checks that the config is well-formed.
func (c *ServerConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("mcp server config: name must not be empty")
	}
	switch c.Transport {
	case TransportStdio, "":
		if c.Command == "" {
			return fmt.Errorf("mcp server %q: command must not be empty for stdio transport", c.Name)
		}
	case TransportSSE:
		if c.URL == "" {
			return fmt.Errorf("mcp server %q: url must not be empty for sse transport", c.Name)
		}
	default:
		return fmt.Errorf("mcp server %q: unknown transport %q", c.Name, c.Transport)
	}
	return nil
}

// ExpandEnv substitutes ${VAR} / $VAR in Command, Args, and Env values.
// Uses ExpandEnvVarsInString for ${VAR:-default} support.
func (c *ServerConfig) ExpandEnv() *ServerConfig {
	cp := *c
	cp.Command = expandEnvSimple(c.Command)
	cp.Args = make([]string, len(c.Args))
	for i, a := range c.Args {
		cp.Args[i] = expandEnvSimple(a)
	}
	cp.Env = make([]string, len(c.Env))
	for i, e := range c.Env {
		cp.Env[i] = expandEnvSimple(e)
	}
	if cp.URL != "" {
		cp.URL = expandEnvSimple(c.URL)
	}
	for k, v := range cp.Headers {
		cp.Headers[k] = expandEnvSimple(v)
	}
	return &cp
}

// expandEnvSimple expands env vars using ExpandEnvVarsInString (supports defaults),
// falling back to os.ExpandEnv for $VAR syntax.
func expandEnvSimple(s string) string {
	result := ExpandEnvVarsInString(s)
	// Also handle bare $VAR references that aren't wrapped in ${...}
	return os.ExpandEnv(result.Expanded)
}

// envVarPattern matches ${VAR} and ${VAR:-default} syntax.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// EnvExpansionResult holds the result of environment variable expansion.
type EnvExpansionResult struct {
	Expanded    string
	MissingVars []string
}

// ExpandEnvVarsInString expands ${VAR} and ${VAR:-default} syntax in a string.
// Returns the expanded string and a list of variables that were referenced but
// not found (and had no default).
// Aligned with claude-code-main expandEnvVarsInString.
func ExpandEnvVarsInString(value string) EnvExpansionResult {
	var missingVars []string

	expanded := envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		// Strip ${...}
		inner := match[2 : len(match)-1]

		// Split on :- to support default values (limit to 2 parts).
		parts := strings.SplitN(inner, ":-", 2)
		varName := parts[0]

		envValue, ok := os.LookupEnv(varName)
		if ok {
			return envValue
		}
		if len(parts) == 2 {
			return parts[1] // default value
		}

		missingVars = append(missingVars, varName)
		return match // leave original if not found
	})

	return EnvExpansionResult{
		Expanded:    expanded,
		MissingVars: missingVars,
	}
}

// GlobalMCPConfig aggregates multiple server configs (mirrors .claude.json mcp section).
type GlobalMCPConfig struct {
	Servers []ServerConfig `json:"mcpServers" yaml:"mcpServers"`
}

// FindServer returns the config for a named server or (nil, false).
func (g *GlobalMCPConfig) FindServer(name string) (*ServerConfig, bool) {
	for i := range g.Servers {
		if g.Servers[i].Name == name {
			return &g.Servers[i], true
		}
	}
	return nil, false
}

// Active returns all non-disabled server configs.
func (g *GlobalMCPConfig) Active() []ServerConfig {
	var out []ServerConfig
	for _, s := range g.Servers {
		if !s.Disabled {
			out = append(out, s)
		}
	}
	return out
}

// ── .mcp.json file loading ──────────────────────────────────────────────────

// McpJsonConfig is the on-disk format for .mcp.json files.
type McpJsonConfig struct {
	McpServers map[string]json.RawMessage `json:"mcpServers"`
}

// LoadMcpJsonFile reads and parses a .mcp.json file.
// Returns the parsed configs keyed by server name, or an error.
func LoadMcpJsonFile(path string) (map[string]ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var raw McpJsonConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	result := make(map[string]ServerConfig, len(raw.McpServers))
	for name, rawCfg := range raw.McpServers {
		var cfg ServerConfig
		if err := json.Unmarshal(rawCfg, &cfg); err != nil {
			return nil, fmt.Errorf("parse server %q in %s: %w", name, path, err)
		}
		cfg.Name = name
		// Default transport to stdio if not specified.
		if cfg.Transport == "" {
			cfg.Transport = TransportStdio
		}
		result[name] = cfg
	}
	return result, nil
}

// ── Multi-scope config merging ──────────────────────────────────────────────

// ScopedConfigSource is a set of server configs from a single scope.
type ScopedConfigSource struct {
	Scope   ConfigScope
	Servers map[string]ServerConfig
}

// MergeConfigs merges multiple scoped config sources into a GlobalMCPConfig.
// Later sources override earlier ones if they share the same server name.
// The resulting servers carry their scope in the Name field for traceability.
func MergeConfigs(sources ...ScopedConfigSource) GlobalMCPConfig {
	merged := make(map[string]ServerConfig)
	for _, src := range sources {
		for name, cfg := range src.Servers {
			cfg.Name = name
			merged[name] = cfg
		}
	}
	var out GlobalMCPConfig
	for _, cfg := range merged {
		out.Servers = append(out.Servers, cfg)
	}
	return out
}

// MergeScopedConfigs merges multiple scoped config sources into ScopedServerConfigs.
// Later sources override earlier ones if they share the same server name.
func MergeScopedConfigs(sources ...ScopedConfigSource) map[string]ScopedServerConfig {
	merged := make(map[string]ScopedServerConfig)
	for _, src := range sources {
		for name, cfg := range src.Servers {
			cfg.Name = name
			merged[name] = ScopedServerConfig{
				ServerConfig: cfg,
				Scope:        src.Scope,
			}
		}
	}
	return merged
}

// IsMcpServerDisabled checks if a server is disabled by name in a disabled set.
func IsMcpServerDisabled(name string, disabledServers map[string]bool) bool {
	return disabledServers[name]
}
