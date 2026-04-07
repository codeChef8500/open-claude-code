package teammem

import (
	"regexp"
	"strings"
)

// ────────────────────────────────────────────────────────────────────────────
// Secret scanner — aligned with claude-code-main
// src/services/teamMemorySync/secretScanner.ts
// ────────────────────────────────────────────────────────────────────────────

// SecretMatch records a detected secret in content.
type SecretMatch struct {
	RuleID     string
	Matched    string // redacted portion
	StartIndex int
}

// ScanForSecrets scans content for potential secrets/credentials.
// Returns a list of matches (empty if clean).
func ScanForSecrets(content string) []SecretMatch {
	var matches []SecretMatch
	for _, rule := range secretRules {
		locs := rule.Pattern.FindAllStringIndex(content, -1)
		for _, loc := range locs {
			matched := content[loc[0]:loc[1]]
			// Redact for logging: show first 4 and last 4 chars
			redacted := redactSecret(matched)
			matches = append(matches, SecretMatch{
				RuleID:     rule.ID,
				Matched:    redacted,
				StartIndex: loc[0],
			})
		}
	}
	return matches
}

// FilterSecretsFromEntries filters out entries containing secrets.
// Returns (clean entries, skipped entries with reasons).
func FilterSecretsFromEntries(entries []MemoryEntry) ([]MemoryEntry, []SecretSkippedFile) {
	var clean []MemoryEntry
	var skipped []SecretSkippedFile

	for _, e := range entries {
		matches := ScanForSecrets(e.Content)
		if len(matches) > 0 {
			reasons := make([]string, 0, len(matches))
			for _, m := range matches {
				reasons = append(reasons, m.RuleID)
			}
			skipped = append(skipped, SecretSkippedFile{
				Key:    e.Key,
				Reason: strings.Join(reasons, ", "),
			})
		} else {
			clean = append(clean, e)
		}
	}

	return clean, skipped
}

// ── Secret detection rules ─────────────────────────────────────────────────

type secretRule struct {
	ID      string
	Pattern *regexp.Regexp
}

var secretRules = []secretRule{
	// AWS
	{ID: "aws-access-key", Pattern: regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`)},
	{ID: "aws-secret-key", Pattern: regexp.MustCompile(`(?i)(?:aws_secret_access_key|aws_secret_key)\s*[=:]\s*[A-Za-z0-9/+=]{40}`)},

	// GitHub
	{ID: "github-pat", Pattern: regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`)},
	{ID: "github-oauth", Pattern: regexp.MustCompile(`gho_[A-Za-z0-9]{36}`)},
	{ID: "github-app", Pattern: regexp.MustCompile(`(?:ghs|ghr)_[A-Za-z0-9]{36}`)},
	{ID: "github-fine-grained", Pattern: regexp.MustCompile(`github_pat_[A-Za-z0-9_]{82}`)},

	// GitLab
	{ID: "gitlab-pat", Pattern: regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`)},

	// Slack
	{ID: "slack-token", Pattern: regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{10,}`)},
	{ID: "slack-webhook", Pattern: regexp.MustCompile(`https://hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]+`)},

	// Generic API keys
	{ID: "generic-api-key", Pattern: regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|api[_-]?secret)\s*[=:]\s*['"]?[A-Za-z0-9_-]{20,}['"]?`)},
	{ID: "generic-secret", Pattern: regexp.MustCompile(`(?i)(?:secret[_-]?key|client[_-]?secret|access[_-]?secret)\s*[=:]\s*['"]?[A-Za-z0-9_/+=]{20,}['"]?`)},

	// Private keys
	{ID: "private-key", Pattern: regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`)},

	// Google
	{ID: "google-api-key", Pattern: regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)},

	// Stripe
	{ID: "stripe-key", Pattern: regexp.MustCompile(`(?:sk|pk)_(?:live|test)_[A-Za-z0-9]{20,}`)},

	// Anthropic
	{ID: "anthropic-key", Pattern: regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},

	// OpenAI
	{ID: "openai-key", Pattern: regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`)},

	// Generic bearer/password in assignments
	{ID: "bearer-token", Pattern: regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9_-]{20,}`)},
	{ID: "password-assign", Pattern: regexp.MustCompile(`(?i)(?:password|passwd|pwd)\s*[=:]\s*['"]?[^\s'"]{8,}['"]?`)},
}

func redactSecret(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
