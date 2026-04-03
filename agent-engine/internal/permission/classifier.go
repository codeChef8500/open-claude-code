package permission

import (
	"context"
	"strings"
)

// YoloClassifier is a rule-based auto-mode classifier that determines whether
// a tool call should be allowed or blocked without an LLM call. It mirrors
// claude-code-main's yoloClassifier.ts logic.
//
// Classification is done in two stages:
//  1. Fast rule-based check (safe commands whitelist + dangerous patterns)
//  2. If inconclusive, falls through to the LLM-based classifier (if set)
type YoloClassifier struct {
	// rules are user-configurable allow/soft-deny patterns.
	rules AutoModeRules
	// llmClassifier is the optional LLM-based fallback classifier.
	llmClassifier Classifier
	// workDir is the current working directory for path safety checks.
	workDir string
}

// NewYoloClassifier creates a rule-based auto-mode classifier.
func NewYoloClassifier(rules AutoModeRules, workDir string, llmFallback Classifier) *YoloClassifier {
	return &YoloClassifier{
		rules:         rules,
		llmClassifier: llmFallback,
		workDir:       workDir,
	}
}

// Classify evaluates a tool action against safe/dangerous rules.
func (yc *YoloClassifier) Classify(ctx context.Context, req ClassifyRequest) (*ClassifierResult, error) {
	// Stage 1: Rule-based fast path.
	result := yc.classifyRuleBased(req)
	if result != nil {
		return result, nil
	}

	// Stage 2: Safe command whitelist (read-only tools are always safe).
	if isAlwaysSafeTool(req.ToolName) {
		return &ClassifierResult{
			ShouldBlock: false,
			Reason:      "tool is always safe (read-only)",
			Stage:       "rule_based",
		}, nil
	}

	// Stage 3: Check dangerous patterns for bash.
	if isBashTool(req.ToolName) {
		cmd := extractCommandString(req.ToolInput)
		if cmd != "" {
			if reason := classifyBashCommand(cmd, yc.workDir); reason != "" {
				return &ClassifierResult{
					ShouldBlock: true,
					Reason:      reason,
					Stage:       "rule_based",
				}, nil
			}
			// Check if command matches safe bash patterns.
			if isSafeBashCommand(cmd) {
				return &ClassifierResult{
					ShouldBlock: false,
					Reason:      "command matches safe pattern",
					Stage:       "rule_based",
				}, nil
			}
		}
	}

	// Stage 4: Check file write safety.
	if isFileWriteTool(req.ToolName) {
		path := extractPathString(req.ToolInput)
		if path != "" && yc.workDir != "" {
			if !isUnder(path, yc.workDir) {
				return &ClassifierResult{
					ShouldBlock: true,
					Reason:      "file write outside working directory",
					Stage:       "rule_based",
				}, nil
			}
		}
	}

	// Stage 5: User-configured allow patterns.
	action := yc.matchUserRules(req)
	if action == "allow" {
		return &ClassifierResult{
			ShouldBlock: false,
			Reason:      "matched user allow rule",
			Stage:       "rule_based",
		}, nil
	}
	if action == "deny" {
		return &ClassifierResult{
			ShouldBlock: true,
			Reason:      "matched user soft-deny rule",
			Stage:       "rule_based",
		}, nil
	}

	// Stage 6: Fall through to LLM classifier if available.
	if yc.llmClassifier != nil {
		return yc.llmClassifier.Classify(ctx, req)
	}

	// Default: allow (permissive fallback).
	return &ClassifierResult{
		ShouldBlock: false,
		Reason:      "no rule matched, default allow",
		Stage:       "rule_based",
	}, nil
}

// classifyRuleBased checks user-configured rules first.
func (yc *YoloClassifier) classifyRuleBased(req ClassifyRequest) *ClassifierResult {
	// Check explicit deny rules from AutoModeRules.
	for _, pat := range yc.rules.SoftDeny {
		if matchAutoModePattern(pat, req.ToolName, req.ToolInput) {
			return &ClassifierResult{
				ShouldBlock: true,
				Reason:      "matched soft-deny rule: " + pat,
				Stage:       "rule_based",
			}
		}
	}
	return nil
}

// matchUserRules checks user allow/deny patterns and returns "allow", "deny",
// or "" for no match.
func (yc *YoloClassifier) matchUserRules(req ClassifyRequest) string {
	for _, pat := range yc.rules.Allow {
		if matchAutoModePattern(pat, req.ToolName, req.ToolInput) {
			return "allow"
		}
	}
	for _, pat := range yc.rules.SoftDeny {
		if matchAutoModePattern(pat, req.ToolName, req.ToolInput) {
			return "deny"
		}
	}
	return ""
}

// matchAutoModePattern matches a pattern like "Bash(git *)" or "Read" against
// a tool name and input.
func matchAutoModePattern(pattern, toolName string, input interface{}) bool {
	open := strings.IndexByte(pattern, '(')
	if open < 0 {
		return strings.EqualFold(pattern, toolName)
	}
	patTool := pattern[:open]
	if !strings.EqualFold(patTool, toolName) {
		return false
	}
	inner := pattern[open+1:]
	if strings.HasSuffix(inner, ")") {
		inner = inner[:len(inner)-1]
	}
	// Match inner against command or path from input.
	arg := extractCommandString(input)
	if arg == "" {
		arg = extractPathString(input)
	}
	return MatchWildcardPattern(inner, arg)
}

// ── Safe tool/command classification ────────────────────────────────────────

// isAlwaysSafeTool returns true for tools that are purely read-only.
func isAlwaysSafeTool(name string) bool {
	switch strings.ToLower(name) {
	case "read", "fileread", "file_read",
		"glob", "globtool", "glob_tool",
		"grep", "greptool", "grep_tool",
		"toolsearch", "tool_search",
		"taskget", "task_get",
		"tasklist", "task_list",
		"listmcpresources", "list_mcp_resources",
		"readmcpresource", "read_mcp_resource",
		"listpeers", "list_peers",
		"lsp", "lsptool":
		return true
	}
	return false
}

// isBashTool returns true for shell execution tools.
func isBashTool(name string) bool {
	switch strings.ToLower(name) {
	case "bash", "bashtool", "bash_tool",
		"powershell", "powershelltool":
		return true
	}
	return false
}

// isFileWriteTool returns true for tools that write files.
func isFileWriteTool(name string) bool {
	switch strings.ToLower(name) {
	case "write", "filewrite", "file_write",
		"edit", "fileedit", "file_edit",
		"multiedit", "multi_edit",
		"notebookedit", "notebook_edit":
		return true
	}
	return false
}

// safeBashPrefixes are shell command prefixes that are considered safe.
var safeBashPrefixes = []string{
	"git status", "git log", "git diff", "git show", "git branch",
	"git tag", "git remote", "git rev-parse", "git describe",
	"git ls-files", "git ls-tree", "git cat-file",
	"ls", "cat", "head", "tail", "wc", "echo", "printf",
	"pwd", "whoami", "hostname", "date", "which", "type",
	"find", "grep", "rg", "ag", "fd", "fzf",
	"tree", "file", "stat", "du", "df",
	"go version", "go env", "go list",
	"node --version", "npm --version", "npx --version",
	"python --version", "python3 --version", "pip list",
	"cargo --version", "rustc --version",
	"java --version", "javac --version",
	"docker ps", "docker images",
	"kubectl get", "kubectl describe",
}

// isSafeBashCommand returns true if the command matches a known-safe pattern.
func isSafeBashCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	lower := strings.ToLower(cmd)
	for _, safe := range safeBashPrefixes {
		if lower == safe || strings.HasPrefix(lower, safe+" ") || strings.HasPrefix(lower, safe+"\t") {
			return true
		}
	}
	return false
}

// dangerousBashPatterns are shell patterns that should be blocked in auto-mode.
var dangerousBashPatterns = []struct {
	pattern string
	reason  string
}{
	{"rm -rf /", "recursive delete of root"},
	{"rm -rf ~", "recursive delete of home"},
	{"rm -rf .", "recursive delete of current directory"},
	{"> /dev/", "write to device file"},
	{"dd if=", "raw disk write"},
	{"mkfs", "filesystem format"},
	{"chmod 777", "world-writable permission"},
	{"chmod -R 777", "recursive world-writable permission"},
	{"curl | sh", "pipe curl to shell"},
	{"curl | bash", "pipe curl to bash"},
	{"wget -O - | sh", "pipe wget to shell"},
	{"eval $(", "eval of command substitution"},
	{"base64 -d | sh", "execute base64 encoded command"},
	{":(){", "fork bomb"},
	{"shutdown", "system shutdown"},
	{"reboot", "system reboot"},
	{"halt", "system halt"},
	{"poweroff", "system poweroff"},
	{"format c:", "format drive (Windows)"},
	{"del /f /s /q c:", "recursive delete (Windows)"},
}

// classifyBashCommand returns a non-empty reason if the command is dangerous.
func classifyBashCommand(cmd, workDir string) string {
	lower := strings.ToLower(strings.TrimSpace(cmd))

	// Check dangerous patterns.
	for _, dp := range dangerousBashPatterns {
		if strings.Contains(lower, dp.pattern) {
			return "dangerous command: " + dp.reason
		}
	}

	// Check for write redirects to sensitive paths.
	if strings.Contains(cmd, ">") {
		parts := strings.SplitN(cmd, ">", 2)
		if len(parts) == 2 {
			target := strings.TrimSpace(parts[1])
			target = strings.TrimPrefix(target, ">") // handle >>
			target = strings.TrimSpace(target)
			if isSensitivePath(target) {
				return "redirect to sensitive path: " + target
			}
		}
	}

	// Check for pipe to shell.
	if strings.Contains(lower, "| sh") || strings.Contains(lower, "| bash") ||
		strings.Contains(lower, "|sh") || strings.Contains(lower, "|bash") {
		if strings.Contains(lower, "curl") || strings.Contains(lower, "wget") {
			return "pipe download to shell execution"
		}
	}

	return ""
}

// isSensitivePath returns true for paths that should not be written to.
func isSensitivePath(path string) bool {
	path = strings.TrimSpace(path)
	sensitivePrefixes := []string{
		"/etc/", "/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/",
		"/boot/", "/sys/", "/proc/", "/dev/",
		"C:\\Windows\\", "C:\\System32\\",
	}
	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(path, prefix) || strings.EqualFold(path, strings.TrimSuffix(prefix, "/")) {
			return true
		}
	}
	return false
}

// ── Shadow rule detection ───────────────────────────────────────────────────

// ShadowConflict describes a conflict between two permission rules.
type ShadowConflict struct {
	AllowRule Rule   `json:"allow_rule"`
	DenyRule  Rule   `json:"deny_rule"`
	Desc      string `json:"description"`
}

// DetectShadowRules finds cases where an allow rule is shadowed by a deny rule
// or vice versa, which may indicate misconfiguration.
func DetectShadowRules(rules []Rule) []ShadowConflict {
	var conflicts []ShadowConflict
	for i, a := range rules {
		if a.Type != RuleAllow {
			continue
		}
		for j, b := range rules {
			if i == j || b.Type != RuleDeny {
				continue
			}
			// Check if the allow pattern overlaps with the deny pattern.
			if patternsOverlap(a.Pattern, b.Pattern) {
				conflicts = append(conflicts, ShadowConflict{
					AllowRule: a,
					DenyRule:  b,
					Desc:      "allow rule '" + a.Pattern + "' is shadowed by deny rule '" + b.Pattern + "'",
				})
			}
		}
	}
	return conflicts
}

// patternsOverlap returns true if two patterns could match the same string.
func patternsOverlap(a, b string) bool {
	// Exact match.
	if a == b {
		return true
	}
	// If either is a wildcard, they overlap.
	if a == "*" || b == "*" {
		return true
	}
	// Extract tool names from patterns like "Bash(git *)"
	aOpen := strings.IndexByte(a, '(')
	bOpen := strings.IndexByte(b, '(')
	// Both are tool-name-only patterns.
	if aOpen < 0 && bOpen < 0 {
		return strings.EqualFold(a, b)
	}
	// One has arguments, one doesn't — same tool name means overlap.
	aTool := a
	if aOpen >= 0 {
		aTool = a[:aOpen]
	}
	bTool := b
	if bOpen >= 0 {
		bTool = b[:bOpen]
	}
	if !strings.EqualFold(aTool, bTool) {
		return false
	}
	// Same tool, one has no arg constraint → always overlaps.
	if aOpen < 0 || bOpen < 0 {
		return true
	}
	// Both have arg constraints — check if the inner patterns could overlap.
	aInner := a[aOpen+1:]
	if strings.HasSuffix(aInner, ")") {
		aInner = aInner[:len(aInner)-1]
	}
	bInner := b[bOpen+1:]
	if strings.HasSuffix(bInner, ")") {
		bInner = bInner[:len(bInner)-1]
	}
	// If either inner is "*", they overlap.
	if aInner == "*" || bInner == "*" {
		return true
	}
	// Check static prefix overlap.
	aPrefix := ExtractStaticPrefix(aInner)
	bPrefix := ExtractStaticPrefix(bInner)
	return strings.HasPrefix(aPrefix, bPrefix) || strings.HasPrefix(bPrefix, aPrefix)
}

// ── Input extraction helpers ────────────────────────────────────────────────

func extractCommandString(input interface{}) string {
	if m, ok := input.(map[string]interface{}); ok {
		for _, key := range []string{"command", "cmd"} {
			if v, ok := m[key]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

func extractPathString(input interface{}) string {
	if m, ok := input.(map[string]interface{}); ok {
		for _, key := range []string{"path", "file_path", "filePath"} {
			if v, ok := m[key]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}
