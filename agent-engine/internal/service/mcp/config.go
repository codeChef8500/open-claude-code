package mcp

import (
	"fmt"
	"os"
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
func (c *ServerConfig) ExpandEnv() *ServerConfig {
	cp := *c
	cp.Command = os.ExpandEnv(c.Command)
	cp.Args = make([]string, len(c.Args))
	for i, a := range c.Args {
		cp.Args[i] = os.ExpandEnv(a)
	}
	cp.Env = make([]string, len(c.Env))
	for i, e := range c.Env {
		cp.Env[i] = os.ExpandEnv(e)
	}
	return &cp
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
