package agent

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WorktreeManager manages Git worktrees for agent isolation.
type WorktreeManager struct {
	baseDir string // parent directory that contains all worktrees
}

// NewWorktreeManager creates a WorktreeManager that stores worktrees under baseDir.
func NewWorktreeManager(baseDir string) *WorktreeManager {
	return &WorktreeManager{baseDir: baseDir}
}

// CreateWorktree creates a new Git worktree for the given agentID at a
// sub-directory of baseDir.  It uses `git worktree add` on the repoDir repo.
// Returns the path of the new worktree.
func (wm *WorktreeManager) CreateWorktree(agentID, repoDir string) (string, error) {
	if err := os.MkdirAll(wm.baseDir, 0o755); err != nil {
		return "", fmt.Errorf("worktree base dir: %w", err)
	}

	worktreePath := filepath.Join(wm.baseDir, "wt-"+agentID)

	cmd := exec.Command("git", "worktree", "add", "--detach", worktreePath, "HEAD")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, out)
	}

	return worktreePath, nil
}

// RemoveWorktree removes the Git worktree for the given agentID.
func (wm *WorktreeManager) RemoveWorktree(agentID, repoDir string) error {
	worktreePath := filepath.Join(wm.baseDir, "wt-"+agentID)

	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, out)
	}

	return nil
}

// WorktreePath returns the expected worktree path for an agentID.
func (wm *WorktreeManager) WorktreePath(agentID string) string {
	return filepath.Join(wm.baseDir, "wt-"+agentID)
}

// Exists reports whether a worktree directory exists for agentID.
func (wm *WorktreeManager) Exists(agentID string) bool {
	_, err := os.Stat(wm.WorktreePath(agentID))
	return err == nil
}

// WorktreeHasChanges checks if a worktree directory has uncommitted changes.
func WorktreeHasChanges(worktreePath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return len(out) > 0, nil
}

// TouchWorktree updates the modification time of a worktree directory
// to prevent stale cleanup.
func TouchWorktree(worktreePath string) error {
	now := time.Now()
	return os.Chtimes(worktreePath, now, now)
}

// ── Change Detection ──────────────────────────────────────────────────────

// WorktreeChangeSummary holds a summary of changes in a worktree.
type WorktreeChangeSummary struct {
	WorktreePath  string
	ChangedFiles  []string
	AddedFiles    []string
	DeletedFiles  []string
	ModifiedFiles []string
	HasChanges    bool
	DiffStat      string // output of git diff --stat
}

// DetectChanges returns a detailed summary of changes in a worktree.
func DetectChanges(worktreePath string) (*WorktreeChangeSummary, error) {
	summary := &WorktreeChangeSummary{WorktreePath: worktreePath}

	// git status --porcelain for file-level changes.
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		file := strings.TrimSpace(line[2:])

		summary.ChangedFiles = append(summary.ChangedFiles, file)

		switch {
		case strings.Contains(status, "A") || status == "??":
			summary.AddedFiles = append(summary.AddedFiles, file)
		case strings.Contains(status, "D"):
			summary.DeletedFiles = append(summary.DeletedFiles, file)
		case strings.Contains(status, "M"):
			summary.ModifiedFiles = append(summary.ModifiedFiles, file)
		}
	}

	summary.HasChanges = len(summary.ChangedFiles) > 0

	// git diff --stat for a compact summary.
	if summary.HasChanges {
		statCmd := exec.Command("git", "diff", "--stat", "HEAD")
		statCmd.Dir = worktreePath
		statOut, err := statCmd.Output()
		if err == nil {
			summary.DiffStat = strings.TrimSpace(string(statOut))
		}
	}

	return summary, nil
}

// FormatChangeSummary formats a WorktreeChangeSummary as a human-readable string.
func FormatChangeSummary(s *WorktreeChangeSummary) string {
	if !s.HasChanges {
		return "No changes detected."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Changes in %s:\n", s.WorktreePath))
	if len(s.AddedFiles) > 0 {
		sb.WriteString(fmt.Sprintf("  Added:    %d files\n", len(s.AddedFiles)))
	}
	if len(s.ModifiedFiles) > 0 {
		sb.WriteString(fmt.Sprintf("  Modified: %d files\n", len(s.ModifiedFiles)))
	}
	if len(s.DeletedFiles) > 0 {
		sb.WriteString(fmt.Sprintf("  Deleted:  %d files\n", len(s.DeletedFiles)))
	}
	if s.DiffStat != "" {
		sb.WriteString("\n" + s.DiffStat + "\n")
	}
	return sb.String()
}

// ── Cleanup Logic ─────────────────────────────────────────────────────────

// CleanupStaleWorktrees removes worktrees that haven't been modified
// within maxAge. Returns the number of worktrees removed.
func (wm *WorktreeManager) CleanupStaleWorktrees(repoDir string, maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(wm.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read worktree dir: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "wt-") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			agentID := strings.TrimPrefix(e.Name(), "wt-")
			wtPath := filepath.Join(wm.baseDir, e.Name())

			// Check if worktree has uncommitted changes before removing.
			hasChanges, _ := WorktreeHasChanges(wtPath)
			if hasChanges {
				slog.Warn("worktree: stale but has changes, skipping",
					slog.String("agent_id", agentID),
					slog.String("path", wtPath),
				)
				continue
			}

			if err := wm.RemoveWorktree(agentID, repoDir); err != nil {
				slog.Warn("worktree: cleanup failed",
					slog.String("agent_id", agentID),
					slog.Any("err", err),
				)
				continue
			}
			removed++
		}
	}

	// Also prune stale git worktree refs.
	pruneCmd := exec.Command("git", "worktree", "prune")
	pruneCmd.Dir = repoDir
	_ = pruneCmd.Run()

	return removed, nil
}

// ListWorktrees returns all active worktree paths managed by this manager.
func (wm *WorktreeManager) ListWorktrees() ([]string, error) {
	entries, err := os.ReadDir(wm.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "wt-") {
			paths = append(paths, filepath.Join(wm.baseDir, e.Name()))
		}
	}
	return paths, nil
}

// CommitWorktreeChanges commits all changes in a worktree with the given message.
// Used by the coordinator to stage worker results before merging.
func CommitWorktreeChanges(worktreePath, message string) error {
	// Stage all changes.
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = worktreePath
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w\n%s", err, out)
	}

	// Check if there's anything to commit.
	statusCmd := exec.Command("git", "diff", "--cached", "--quiet")
	statusCmd.Dir = worktreePath
	if err := statusCmd.Run(); err == nil {
		return nil // nothing to commit
	}

	// Commit.
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = worktreePath
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, out)
	}

	return nil
}

// GetWorktreeCommitHash returns the HEAD commit hash of a worktree.
func GetWorktreeCommitHash(worktreePath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetMainBranch returns the main/master branch name of the repository.
func GetMainBranch(repoDir string) string {
	// Try common main branch names.
	for _, name := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+name)
		cmd.Dir = repoDir
		if err := cmd.Run(); err == nil {
			return name
		}
	}
	// Fallback: use symbolic-ref.
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return "main"
}
