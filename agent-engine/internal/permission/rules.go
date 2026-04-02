package permission

import (
	"strings"
)

// RuleSource identifies where a permission rule originated.
type RuleSource string

const (
	RuleSourceUser    RuleSource = "user"
	RuleSourceProject RuleSource = "project"
	RuleSourceAgent   RuleSource = "agent"
	RuleSourceMCP     RuleSource = "mcp"
)

// PermissionRule is a single allow or deny entry in the permission config.
type PermissionRule struct {
	Source  RuleSource
	Pattern string // glob-like pattern, e.g. "Bash(git *)"
	Allow   bool   // true=allow, false=deny
}

// RuleSet is an ordered list of PermissionRules evaluated first-match.
type RuleSet []PermissionRule

// Match returns the first matching rule for the given tool name and argument.
// Returns (nil, false) if no rule matches.
func (rs RuleSet) Match(toolName, argument string) (*PermissionRule, bool) {
	for i := range rs {
		r := &rs[i]
		if matchPermissionPattern(r.Pattern, toolName, argument) {
			return r, true
		}
	}
	return nil, false
}

// AllowedByRules reports true if the first matching rule is an allow rule.
func (rs RuleSet) AllowedByRules(toolName, argument string) (bool, bool) {
	r, matched := rs.Match(toolName, argument)
	if !matched {
		return false, false
	}
	return r.Allow, true
}

// GetDenyRuleForTool returns the first deny rule that matches toolName+argument.
func GetDenyRuleForTool(rules RuleSet, toolName, argument string) *PermissionRule {
	for i := range rules {
		r := &rules[i]
		if !r.Allow && matchPermissionPattern(r.Pattern, toolName, argument) {
			return r
		}
	}
	return nil
}

// PermissionRuleExtractPrefix returns the static prefix of a glob-like
// permission pattern, e.g. "Bash(git *)" → "git ".
// This is used to present concise descriptions in the permission UI.
func PermissionRuleExtractPrefix(pattern string) string {
	// Strip "ToolName(" prefix and trailing ")"
	open := strings.IndexByte(pattern, '(')
	if open < 0 {
		return pattern
	}
	inner := pattern[open+1:]
	if strings.HasSuffix(inner, ")") {
		inner = inner[:len(inner)-1]
	}
	// Trim wildcard suffix.
	inner = strings.TrimRight(inner, "*")
	inner = strings.TrimRight(inner, " ")
	return inner
}

// matchPermissionPattern matches a permission rule pattern against a
// (toolName, argument) pair.
//
// Pattern formats:
//   - "Bash"           — any Bash call
//   - "Bash(git *)"    — Bash calls whose argument starts with "git "
//   - "Bash(git status)" — exact Bash argument
//   - "Write(/tmp/*)"  — Write calls to paths under /tmp/
func matchPermissionPattern(pattern, toolName, argument string) bool {
	open := strings.IndexByte(pattern, '(')
	if open < 0 {
		// Tool-name only match.
		return strings.EqualFold(pattern, toolName)
	}
	patternTool := pattern[:open]
	if !strings.EqualFold(patternTool, toolName) {
		return false
	}
	inner := pattern[open+1:]
	if strings.HasSuffix(inner, ")") {
		inner = inner[:len(inner)-1]
	}
	return MatchWildcardPattern(inner, argument)
}
