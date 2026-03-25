package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a bare-minimum git repo with an initial commit on "main".
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("init %v: %s: %v", args, out, err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("commit %v: %s: %v", args, out, err)
		}
	}

	return dir
}

func TestCreateForIssue(t *testing.T) {
	repoPath := initTestRepo(t)
	mgr := NewManager()

	wtPath, err := mgr.CreateForIssue(repoPath, 42)
	if err != nil {
		t.Fatalf("CreateForIssue() error: %v", err)
	}

	expectedSuffix := "issue-42"
	if !strings.HasSuffix(wtPath, expectedSuffix) {
		t.Errorf("worktree path = %q, want suffix %q", wtPath, expectedSuffix)
	}

	// Verify worktree is under the repo path
	if !strings.HasPrefix(wtPath, repoPath) {
		t.Errorf("worktree path %q should be under repo path %q", wtPath, repoPath)
	}

	// Verify the worktree directory exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory does not exist")
	}

	// Verify it contains the README from the initial commit
	readmePath := filepath.Join(wtPath, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("worktree missing README.md from initial commit")
	}

	// Verify the worktree is on its own branch (issue-42), not on main
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = wtPath
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch: %v", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch != "issue-42" {
		t.Errorf("worktree branch = %q, want %q", branch, "issue-42")
	}
}

func TestCreateForIssue_AlreadyExists(t *testing.T) {
	repoPath := initTestRepo(t)
	mgr := NewManager()

	path1, err := mgr.CreateForIssue(repoPath, 1)
	if err != nil {
		t.Fatalf("first CreateForIssue() error: %v", err)
	}

	// Second call should return same path without error
	path2, err := mgr.CreateForIssue(repoPath, 1)
	if err != nil {
		t.Fatalf("second CreateForIssue() error: %v", err)
	}
	if path1 != path2 {
		t.Errorf("paths differ: %q vs %q", path1, path2)
	}
}

func TestMultipleWorktrees_DifferentIssues(t *testing.T) {
	repoPath := initTestRepo(t)
	mgr := NewManager()

	wt1, err := mgr.CreateForIssue(repoPath, 1)
	if err != nil {
		t.Fatalf("CreateForIssue(1) error: %v", err)
	}

	wt2, err := mgr.CreateForIssue(repoPath, 2)
	if err != nil {
		t.Fatalf("CreateForIssue(2) error: %v", err)
	}

	if wt1 == wt2 {
		t.Error("different issues should produce different worktree paths")
	}

	// Both should exist
	for _, p := range []string{wt1, wt2} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("worktree %q should exist", p)
		}
	}

	// Each should be on its own branch
	for _, tc := range []struct {
		path, branch string
	}{
		{wt1, "issue-1"},
		{wt2, "issue-2"},
	} {
		cmd := exec.Command("git", "branch", "--show-current")
		cmd.Dir = tc.path
		out, _ := cmd.Output()
		if strings.TrimSpace(string(out)) != tc.branch {
			t.Errorf("worktree %q: branch = %q, want %q", tc.path, strings.TrimSpace(string(out)), tc.branch)
		}
	}

	// Clean up
	mgr.Remove(repoPath, wt1)
	mgr.Remove(repoPath, wt2)
}

func TestRemove(t *testing.T) {
	repoPath := initTestRepo(t)
	mgr := NewManager()

	wtPath, err := mgr.CreateForIssue(repoPath, 99)
	if err != nil {
		t.Fatalf("CreateForIssue() error: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatal("worktree should exist before removal")
	}

	err = mgr.Remove(repoPath, wtPath)
	if err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}
}

func TestGetDefaultBranch_Main(t *testing.T) {
	repoPath := initTestRepo(t) // Creates repo with "main" branch
	branch, err := getDefaultBranch(repoPath)
	if err != nil {
		t.Fatalf("getDefaultBranch() error: %v", err)
	}
	if branch != "main" {
		t.Errorf("branch = %q, want 'main'", branch)
	}
}

func TestGetDefaultBranch_Master(t *testing.T) {
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", "-b", "master"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s: %v", args, out, err)
		}
	}
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644)
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.CombinedOutput()
	}

	branch, err := getDefaultBranch(dir)
	if err != nil {
		t.Fatalf("getDefaultBranch() error: %v", err)
	}
	if branch != "master" {
		t.Errorf("branch = %q, want 'master'", branch)
	}
}

func TestGetDefaultBranch_NoBranch(t *testing.T) {
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", "-b", "develop"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.CombinedOutput()
	}
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644)
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.CombinedOutput()
	}

	_, err := getDefaultBranch(dir)
	if err == nil {
		t.Fatal("expected error when no main/master branch exists")
	}
}

func TestCreateForIssue_InvalidRepoPath(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.CreateForIssue("/nonexistent/repo", 1)
	if err == nil {
		t.Fatal("expected error for invalid repo path")
	}
}

// CreateForPR requires a GitHub remote and `gh` auth, so we only test the
// worktree creation (detached) part locally and verify that `gh pr checkout`
// failure cleans up the worktree.
func TestCreateForPR_NoRemote(t *testing.T) {
	repoPath := initTestRepo(t)
	mgr := NewManager()

	// This will create a detached worktree but `gh pr checkout` will fail
	// because there's no remote/origin. Verify the worktree gets cleaned up.
	_, err := mgr.CreateForPR(repoPath, 123)
	if err == nil {
		t.Fatal("expected error when no remote exists for gh pr checkout")
	}

	// The worktree should have been cleaned up on failure
	wtPath := filepath.Join(repoPath, "review-123")
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Error("worktree should be cleaned up after gh pr checkout failure")
	}
}
