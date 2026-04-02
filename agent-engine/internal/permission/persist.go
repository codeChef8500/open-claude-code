package permission

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PermissionStore persists permission rules to disk across sessions.
type PermissionStore struct {
	mu   sync.RWMutex
	path string
	data StoredPermissions
}

// StoredPermissions is the on-disk format for persisted rules.
type StoredPermissions struct {
	Version      int              `json:"version"`
	AllowRules   []StoredRule     `json:"allow_rules,omitempty"`
	DenyRules    []StoredRule     `json:"deny_rules,omitempty"`
	DeniedCmds   []string         `json:"denied_commands,omitempty"`
	AllowedDirs  []string         `json:"allowed_directories,omitempty"`
	ModeOverride string           `json:"mode_override,omitempty"`
	MCPRules     []MCPPermission  `json:"mcp_rules,omitempty"`
}

// StoredRule is a serializable permission rule.
type StoredRule struct {
	Pattern string     `json:"pattern"`
	Source  RuleSource `json:"source,omitempty"`
	Note    string     `json:"note,omitempty"`
}

// MCPPermission controls access to a specific MCP server or tool.
type MCPPermission struct {
	ServerName string `json:"server_name"`
	ToolName   string `json:"tool_name,omitempty"` // empty = all tools on server
	Allow      bool   `json:"allow"`
	Source     string `json:"source,omitempty"`
}

// NewPermissionStore creates a store backed by the given JSON file.
func NewPermissionStore(path string) *PermissionStore {
	return &PermissionStore{
		path: path,
		data: StoredPermissions{Version: 1},
	}
}

// DefaultPermissionStorePath returns the default path for the permission store.
func DefaultPermissionStorePath(workDir string) string {
	return filepath.Join(workDir, ".claude", "permissions.json")
}

// GlobalPermissionStorePath returns the path for global permission store.
func GlobalPermissionStorePath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".claude", "permissions.json")
}

// Load reads permissions from disk.
func (s *PermissionStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = StoredPermissions{Version: 1}
			return nil
		}
		return fmt.Errorf("load permissions: %w", err)
	}

	var stored StoredPermissions
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("parse permissions: %w", err)
	}
	s.data = stored
	return nil
}

// Save writes permissions to disk.
func (s *PermissionStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("save permissions: mkdir: %w", err)
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("save permissions: marshal: %w", err)
	}

	return os.WriteFile(s.path, data, 0600)
}

// AddAllowRule adds an allow rule and persists.
func (s *PermissionStore) AddAllowRule(pattern string, source RuleSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Deduplicate.
	for _, r := range s.data.AllowRules {
		if r.Pattern == pattern {
			return nil
		}
	}
	s.data.AllowRules = append(s.data.AllowRules, StoredRule{
		Pattern: pattern,
		Source:  source,
	})
	return s.saveLocked()
}

// AddDenyRule adds a deny rule and persists.
func (s *PermissionStore) AddDenyRule(pattern string, source RuleSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range s.data.DenyRules {
		if r.Pattern == pattern {
			return nil
		}
	}
	s.data.DenyRules = append(s.data.DenyRules, StoredRule{
		Pattern: pattern,
		Source:  source,
	})
	return s.saveLocked()
}

// RemoveAllowRule removes an allow rule by pattern.
func (s *PermissionStore) RemoveAllowRule(pattern string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.data.AllowRules[:0]
	for _, r := range s.data.AllowRules {
		if r.Pattern != pattern {
			filtered = append(filtered, r)
		}
	}
	s.data.AllowRules = filtered
	return s.saveLocked()
}

// RemoveDenyRule removes a deny rule by pattern.
func (s *PermissionStore) RemoveDenyRule(pattern string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.data.DenyRules[:0]
	for _, r := range s.data.DenyRules {
		if r.Pattern != pattern {
			filtered = append(filtered, r)
		}
	}
	s.data.DenyRules = filtered
	return s.saveLocked()
}

// AllowRules returns all stored allow rules.
func (s *PermissionStore) AllowRules() []StoredRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]StoredRule, len(s.data.AllowRules))
	copy(out, s.data.AllowRules)
	return out
}

// DenyRules returns all stored deny rules.
func (s *PermissionStore) DenyRules() []StoredRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]StoredRule, len(s.data.DenyRules))
	copy(out, s.data.DenyRules)
	return out
}

// SetMCPPermission adds or updates an MCP permission.
func (s *PermissionStore) SetMCPPermission(perm MCPPermission) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Replace existing entry for same server+tool.
	for i, p := range s.data.MCPRules {
		if p.ServerName == perm.ServerName && p.ToolName == perm.ToolName {
			s.data.MCPRules[i] = perm
			return s.saveLocked()
		}
	}
	s.data.MCPRules = append(s.data.MCPRules, perm)
	return s.saveLocked()
}

// CheckMCPPermission checks if an MCP server/tool is allowed.
// Returns (allowed, matched). If not matched, the caller should use default logic.
func (s *PermissionStore) CheckMCPPermission(serverName, toolName string) (bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check specific tool rule first.
	for _, p := range s.data.MCPRules {
		if p.ServerName == serverName && p.ToolName == toolName {
			return p.Allow, true
		}
	}
	// Check server-wide rule.
	for _, p := range s.data.MCPRules {
		if p.ServerName == serverName && p.ToolName == "" {
			return p.Allow, true
		}
	}
	return false, false
}

// ToRules converts stored rules to the checker's Rule type.
func (s *PermissionStore) ToRules() (allow, deny []Rule) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.data.AllowRules {
		allow = append(allow, Rule{
			Type:    RuleAllow,
			Pattern: r.Pattern,
			Source:  r.Source,
		})
	}
	for _, r := range s.data.DenyRules {
		deny = append(deny, Rule{
			Type:    RuleDeny,
			Pattern: r.Pattern,
			Source:  r.Source,
		})
	}
	return
}

// Summary returns a human-readable summary of stored permissions.
func (s *PermissionStore) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("Permission Configuration:\n")

	if s.data.ModeOverride != "" {
		sb.WriteString(fmt.Sprintf("  Mode override: %s\n", s.data.ModeOverride))
	}

	if len(s.data.AllowRules) > 0 {
		sb.WriteString("  Allow rules:\n")
		for _, r := range s.data.AllowRules {
			sb.WriteString(fmt.Sprintf("    + %s", r.Pattern))
			if r.Source != "" {
				sb.WriteString(fmt.Sprintf(" (from %s)", r.Source))
			}
			sb.WriteString("\n")
		}
	}

	if len(s.data.DenyRules) > 0 {
		sb.WriteString("  Deny rules:\n")
		for _, r := range s.data.DenyRules {
			sb.WriteString(fmt.Sprintf("    - %s", r.Pattern))
			if r.Source != "" {
				sb.WriteString(fmt.Sprintf(" (from %s)", r.Source))
			}
			sb.WriteString("\n")
		}
	}

	if len(s.data.MCPRules) > 0 {
		sb.WriteString("  MCP permissions:\n")
		for _, p := range s.data.MCPRules {
			action := "allow"
			if !p.Allow {
				action = "deny"
			}
			if p.ToolName != "" {
				sb.WriteString(fmt.Sprintf("    %s: %s/%s\n", action, p.ServerName, p.ToolName))
			} else {
				sb.WriteString(fmt.Sprintf("    %s: %s (all tools)\n", action, p.ServerName))
			}
		}
	}

	if len(s.data.AllowRules) == 0 && len(s.data.DenyRules) == 0 && len(s.data.MCPRules) == 0 {
		sb.WriteString("  (no custom rules configured)\n")
	}

	return sb.String()
}

func (s *PermissionStore) saveLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
