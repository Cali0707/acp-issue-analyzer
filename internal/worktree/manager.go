package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

// CreateForIssue creates a worktree with a new branch (e.g. "issue-42")
// based off the repo's default branch. The worktree is created as a sibling
// directory under the bare repo at repoPath.
func (m *Manager) CreateForIssue(repoPath string, issueNumber int) (string, error) {
	wtName := fmt.Sprintf("issue-%d", issueNumber)
	wtPath := filepath.Join(repoPath, wtName)

	if _, err := os.Stat(wtPath); err == nil {
		return wtPath, nil // already exists
	}

	defaultBranch, err := getDefaultBranch(repoPath)
	if err != nil {
		return "", err
	}

	// Create worktree with a new branch from the default branch.
	cmd := exec.Command("git", "worktree", "add", "-b", wtName, wtPath, defaultBranch)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return wtPath, nil
}

// CreateForPR creates a worktree and checks out the PR into it using `gh pr checkout`.
// The worktree is created as a sibling directory under the bare repo at repoPath.
func (m *Manager) CreateForPR(repoPath string, prNumber int) (string, error) {
	wtName := fmt.Sprintf("review-%d", prNumber)
	wtPath := filepath.Join(repoPath, wtName)

	if _, err := os.Stat(wtPath); err == nil {
		return wtPath, nil // already exists
	}

	defaultBranch, err := getDefaultBranch(repoPath)
	if err != nil {
		return "", err
	}

	// Create a detached worktree so we don't conflict with any branch name.
	wtCmd := exec.Command("git", "worktree", "add", "--detach", wtPath, defaultBranch)
	wtCmd.Dir = repoPath
	if out, err := wtCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Check out the PR inside the worktree.
	prCmd := exec.Command("gh", "pr", "checkout", fmt.Sprintf("%d", prNumber))
	prCmd.Dir = wtPath
	if out, err := prCmd.CombinedOutput(); err != nil {
		// Clean up the worktree on failure.
		_ = m.Remove(repoPath, wtPath)
		return "", fmt.Errorf("gh pr checkout: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return wtPath, nil
}

// Remove removes a worktree.
func (m *Manager) Remove(repoPath, wtPath string) error {
	cmd := exec.Command("git", "worktree", "remove", wtPath, "--force")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func getDefaultBranch(repoPath string) (string, error) {
	// Try to determine default branch from origin/HEAD
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		// refs/remotes/origin/main -> main
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}
	// Fallback: try common branch names
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		cmd.Dir = repoPath
		if err := cmd.Run(); err == nil {
			return branch, nil
		}
	}
	return "", fmt.Errorf("could not determine default branch for %s", repoPath)
}
