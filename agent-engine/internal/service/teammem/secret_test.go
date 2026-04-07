package teammem

import (
	"testing"
)

func TestScanForSecrets(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantHit bool
		ruleID  string
	}{
		{"clean text", "This is normal memory content about project architecture.", false, ""},
		{"AWS access key", "Use key AKIAIOSFODNN7EXAMPLE for S3", true, "aws-access-key"},
		{"GitHub PAT", "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef1234", true, "github-pat"},
		{"private key header", "-----BEGIN RSA PRIVATE KEY-----", true, "private-key"},
		{"Slack token", "SLACK_TOKEN=xoxb-1234567890-abcdefghij", true, "slack-token"},
		{"generic API key", "api_key = ABCDEFGHIJKLMNOPQRSTUVWXYZ", true, "generic-api-key"},
		{"Stripe key", "sk_live_ABCDEFGHIJKLMNOPQRSTu", true, "stripe-key"},
		{"bearer token", "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", true, "bearer-token"},
		{"password assign", "password = mysuperSecretPass123!", true, "password-assign"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := ScanForSecrets(tt.content)
			if tt.wantHit && len(matches) == 0 {
				t.Errorf("expected secret match for %q", tt.name)
			}
			if !tt.wantHit && len(matches) > 0 {
				t.Errorf("unexpected secret match for %q: %v", tt.name, matches)
			}
			if tt.wantHit && len(matches) > 0 && matches[0].RuleID != tt.ruleID {
				t.Errorf("expected ruleID %q, got %q", tt.ruleID, matches[0].RuleID)
			}
		})
	}
}

func TestFilterSecretsFromEntries(t *testing.T) {
	entries := []MemoryEntry{
		{Key: "clean.md", Content: "Normal project architecture notes."},
		{Key: "has-secret.md", Content: "Use AKIAIOSFODNN7EXAMPLE for AWS access."},
		{Key: "also-clean.md", Content: "Team conventions and coding standards."},
	}

	clean, skipped := FilterSecretsFromEntries(entries)
	if len(clean) != 2 {
		t.Errorf("expected 2 clean entries, got %d", len(clean))
	}
	if len(skipped) != 1 {
		t.Errorf("expected 1 skipped entry, got %d", len(skipped))
	}
	if len(skipped) > 0 && skipped[0].Key != "has-secret.md" {
		t.Errorf("expected skipped key 'has-secret.md', got %q", skipped[0].Key)
	}
}

func TestRedactSecret(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "****"},
		{"AKIAIOSFODNN7EXAMPLE", "AKIA****MPLE"},
	}
	for _, tt := range tests {
		got := redactSecret(tt.input)
		if got != tt.want {
			t.Errorf("redactSecret(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
