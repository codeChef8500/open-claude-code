package permission

import (
	"path/filepath"
	"strings"
)

// MatchWildcardPattern returns true if text matches a shell-style wildcard
// pattern.  Supported metacharacters:
//
//   - '*'  matches any sequence of non-separator characters (like glob)
//   - '**' matches any sequence including path separators
//   - '?'  matches any single character
//
// The match is case-sensitive on Unix and case-insensitive on Windows via
// filepath.Match semantics.
func MatchWildcardPattern(pattern, text string) bool {
	if pattern == "" {
		return text == ""
	}
	// Fast path: no wildcards.
	if !strings.ContainsAny(pattern, "*?[") {
		return pattern == text
	}
	// Use filepath.Match for single-star globs (standard shell glob).
	// For "**" we use a prefix match on the non-glob part.
	if strings.Contains(pattern, "**") {
		return matchDoublestar(pattern, text)
	}
	matched, err := filepath.Match(pattern, text)
	if err != nil {
		// Invalid pattern — fall back to prefix match on the literal part.
		prefix := pattern[:strings.IndexAny(pattern, "*?[")]
		return strings.HasPrefix(text, prefix)
	}
	return matched
}

// matchDoublestar handles patterns containing "**".
// It converts "**" to a simple "match rest of string" operation.
func matchDoublestar(pattern, text string) bool {
	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0]
	suffix := parts[1]

	if !strings.HasPrefix(text, prefix) {
		return false
	}
	rest := text[len(prefix):]

	if suffix == "" || suffix == "/" {
		return true
	}
	// suffix may itself contain more wildcards — recurse.
	if strings.HasPrefix(suffix, "/") {
		suffix = suffix[1:]
	}
	// Try matching suffix against every position in rest.
	for i := 0; i <= len(rest); i++ {
		if MatchWildcardPattern(suffix, rest[i:]) {
			return true
		}
		if i < len(rest) && rest[i] == '/' {
			// Skip ahead to next segment.
		}
	}
	return false
}

// ExtractStaticPrefix returns the longest static (wildcard-free) prefix of a
// shell pattern.  Used to present concise UI descriptions.
func ExtractStaticPrefix(pattern string) string {
	idx := strings.IndexAny(pattern, "*?[")
	if idx < 0 {
		return pattern
	}
	return pattern[:idx]
}

// HasCdCommand reports whether a shell command string contains a bare "cd"
// that would change directory.
func HasCdCommand(command string) bool {
	lc := strings.ToLower(command)
	// Trim leading whitespace and check for "cd " or "cd\t" or just "cd".
	for _, token := range []string{"cd ", "cd\t", " cd ", "\tcd ", ";cd ", "&&cd ", "||cd "} {
		if strings.Contains(lc, token) {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimSpace(lc), "cd") &&
		(len(strings.TrimSpace(lc)) == 2 || lc[2] == ' ' || lc[2] == '\t')
}
