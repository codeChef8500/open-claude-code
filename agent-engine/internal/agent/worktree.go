package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
