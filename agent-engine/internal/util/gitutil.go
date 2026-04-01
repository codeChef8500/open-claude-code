package util

import (
	"context"
	"strings"
)

// GitDiff holds the parsed result of a git diff operation.
type GitDiff struct {
	FilePath     string
	RawDiff      string
	LinesAdded   int
	LinesRemoved int
}

// FetchSingleFileGitDiff runs `git diff HEAD -- <filePath>` and returns the
// parsed diff, or nil if the file has no uncommitted changes.
func FetchSingleFileGitDiff(ctx context.Context, filePath, cwd string) (*GitDiff, error) {
	result, err := Exec(ctx, "git diff HEAD -- "+ShellQuote(filePath), &ExecOptions{CWD: cwd})
	if err != nil {
		return nil, err
	}
	if result.Stdout == "" {
		return nil, nil
	}
	diff := ParseUnifiedDiff(result.Stdout, filePath)
	return diff, nil
}

// ParseUnifiedDiff parses a unified diff string and counts added/removed lines.
func ParseUnifiedDiff(raw, filePath string) *GitDiff {
	diff := &GitDiff{FilePath: filePath, RawDiff: raw}
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			diff.LinesAdded++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			diff.LinesRemoved++
		}
	}
	return diff
}

// CountLinesChanged counts added and removed lines between oldContent and newContent.
func CountLinesChanged(oldContent, newContent string) (added, removed int) {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	oldSet := make(map[string]int)
	for _, l := range oldLines {
		oldSet[l]++
	}
	newSet := make(map[string]int)
	for _, l := range newLines {
		newSet[l]++
	}

	for l, cnt := range newSet {
		if oldSet[l] < cnt {
			added += cnt - oldSet[l]
		}
	}
	for l, cnt := range oldSet {
		if newSet[l] < cnt {
			removed += cnt - newSet[l]
		}
	}
	return
}

// ShellQuote wraps a string in single quotes for safe shell interpolation.
// Single quotes inside the string are escaped via: ' -> '\''
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// GitGetRemoteURL returns the remote URL for the given remote name ("origin" by default).
func GitGetRemoteURL(ctx context.Context, remote, cwd string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	result, err := Exec(ctx, "git remote get-url "+remote, &ExecOptions{CWD: cwd})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

// GitIsRepo reports whether cwd is inside a git repository.
func GitIsRepo(ctx context.Context, cwd string) bool {
	result, err := Exec(ctx, "git rev-parse --is-inside-work-tree", &ExecOptions{CWD: cwd})
	if err != nil {
		return false
	}
	return strings.TrimSpace(result.Stdout) == "true"
}
