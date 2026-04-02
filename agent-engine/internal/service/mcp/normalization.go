package mcp

import (
	"regexp"
	"strings"
)

// claudeAIServerPrefix is prepended to claude.ai-sourced server names.
const claudeAIServerPrefix = "claude.ai "

// invalidNameChars matches any character not in [a-zA-Z0-9_-].
var invalidNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// consecutiveUnderscores collapses runs of underscores.
var consecutiveUnderscores = regexp.MustCompile(`_+`)

// NormalizeNameForMCP normalizes server/tool names to the API pattern
// ^[a-zA-Z0-9_-]{1,64}$.  Replaces invalid characters with underscores.
// For claude.ai servers, also collapses consecutive underscores and strips
// leading/trailing underscores to prevent interference with the __ delimiter.
// Aligned with claude-code-main normalizeNameForMCP.
func NormalizeNameForMCP(name string) string {
	normalized := invalidNameChars.ReplaceAllString(name, "_")
	if strings.HasPrefix(name, claudeAIServerPrefix) {
		normalized = consecutiveUnderscores.ReplaceAllString(normalized, "_")
		normalized = strings.Trim(normalized, "_")
	}
	return normalized
}

// mcpToolDelimiter separates the "mcp", server name, and tool name.
const mcpToolDelimiter = "__"

// McpToolInfo holds parsed MCP tool name components.
type McpToolInfo struct {
	ServerName string
	ToolName   string
}

// McpInfoFromString parses a fully qualified MCP tool name of the form
// "mcp__serverName__toolName" and returns the components.
// Returns nil if the string is not a valid MCP tool name.
//
// Known limitation: if a server name contains "__", parsing will be incorrect.
func McpInfoFromString(toolString string) *McpToolInfo {
	parts := strings.SplitN(toolString, mcpToolDelimiter, 3)
	if len(parts) < 2 || parts[0] != "mcp" || parts[1] == "" {
		return nil
	}
	info := &McpToolInfo{ServerName: parts[1]}
	if len(parts) == 3 && parts[2] != "" {
		info.ToolName = parts[2]
	}
	return info
}

// GetMcpPrefix returns the MCP tool name prefix for a server.
// e.g. "mcp__myserver__"
func GetMcpPrefix(serverName string) string {
	return "mcp" + mcpToolDelimiter + NormalizeNameForMCP(serverName) + mcpToolDelimiter
}

// BuildMcpToolName builds a fully qualified MCP tool name from server and tool names.
// Inverse of McpInfoFromString.
// e.g. BuildMcpToolName("my-server", "read_file") => "mcp__my-server__read_file"
func BuildMcpToolName(serverName, toolName string) string {
	return GetMcpPrefix(serverName) + NormalizeNameForMCP(toolName)
}

// GetMcpDisplayName strips the MCP prefix from a fully qualified tool name
// to return just the tool display name.
func GetMcpDisplayName(fullName, serverName string) string {
	prefix := GetMcpPrefix(serverName)
	return strings.TrimPrefix(fullName, prefix)
}

// GetToolNameForPermissionCheck returns the name to use for permission rule
// matching.  For MCP tools, uses the fully qualified mcp__server__tool name
// so that deny rules targeting builtins don't match MCP tools with the same
// display name.
func GetToolNameForPermissionCheck(name string, mcpInfo *McpToolInfo) string {
	if mcpInfo != nil {
		return BuildMcpToolName(mcpInfo.ServerName, mcpInfo.ToolName)
	}
	return name
}
