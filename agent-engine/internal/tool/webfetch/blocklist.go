package webfetch

import "strings"

// blockedHosts are domains that should never be fetched, matching
// claude-code-main's checkDomainBlocklist.
// These are API endpoints that could leak credentials or cause unintended actions.
var blockedHosts = map[string]bool{
	"api.anthropic.com":     true,
	"api.openai.com":        true,
	"api.stripe.com":        true,
	"hooks.slack.com":       true,
	"discord.com":           true,
	"discordapp.com":        true,
	"login.microsoftonline.com": true,
	"accounts.google.com":   true,
	"oauth.pstmn.io":       true,
}

// CheckDomainBlocklist returns an error message if the hostname is blocked.
// Returns "" if the host is allowed.
func CheckDomainBlocklist(hostname string) string {
	host := strings.ToLower(hostname)
	if blockedHosts[host] {
		return "This domain is blocked for security reasons: " + hostname
	}
	// Also block any subdomain of blocked hosts.
	for blocked := range blockedHosts {
		if strings.HasSuffix(host, "."+blocked) {
			return "This domain is blocked for security reasons: " + hostname
		}
	}
	return ""
}
