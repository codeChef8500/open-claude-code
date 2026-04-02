package prompt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitContext holds git repository information for system prompt injection.
type GitContext struct {
	IsRepo        bool
	Branch        string
	RemoteURL     string
	HasUncommited bool
	RecentCommits []string // most recent N oneline commits
}

// DetectGitContext gathers git info for the given directory.
// Returns a zero GitContext (IsRepo=false) if not in a git repo.
func DetectGitContext(workDir string) GitContext {
	ctx := GitContext{}

	// Check if .git exists anywhere up the tree.
	if !isGitRepo(workDir) {
		return ctx
	}
	ctx.IsRepo = true

	// Current branch.
	if out, err := gitCmd(workDir, "branch", "--show-current"); err == nil {
		ctx.Branch = strings.TrimSpace(out)
	}

	// Remote URL (best-effort).
	if out, err := gitCmd(workDir, "config", "--get", "remote.origin.url"); err == nil {
		ctx.RemoteURL = strings.TrimSpace(out)
	}

	// Uncommitted changes.
	if out, err := gitCmd(workDir, "status", "--porcelain"); err == nil {
		ctx.HasUncommited = strings.TrimSpace(out) != ""
	}

	// Recent commits (last 5).
	if out, err := gitCmd(workDir, "log", "--oneline", "-5"); err == nil {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, l := range lines {
			if l != "" {
				ctx.RecentCommits = append(ctx.RecentCommits, l)
			}
		}
	}

	return ctx
}

// Render formats git context as a prompt section.
func (g GitContext) Render() string {
	if !g.IsRepo {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<git_context>\n")
	if g.Branch != "" {
		sb.WriteString("branch: ")
		sb.WriteString(g.Branch)
		sb.WriteString("\n")
	}
	if g.RemoteURL != "" {
		sb.WriteString("remote: ")
		sb.WriteString(g.RemoteURL)
		sb.WriteString("\n")
	}
	if g.HasUncommited {
		sb.WriteString("uncommitted_changes: true\n")
	}
	if len(g.RecentCommits) > 0 {
		sb.WriteString("recent_commits:\n")
		for _, c := range g.RecentCommits {
			sb.WriteString("  - ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("</git_context>")
	return sb.String()
}

func isGitRepo(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
