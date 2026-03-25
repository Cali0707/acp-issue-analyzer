package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cmurray/acp-issue-analyzer/internal/workflow"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return s
}

func testSession(id string) *PersistedSession {
	return &PersistedSession{
		ID:             id,
		WorkflowType:   workflow.WorkflowBug,
		Owner:          "org",
		Repo:           "repo",
		IssueNumber:    42,
		IssueTitle:     "Test issue",
		AgentName:      "claude",
		AgentSessionID: "agent-sess-1",
		WorktreePath:   "/tmp/worktrees/org-repo-issue-42",
		Status:         StatusCompleted,
		StartedAt:      time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	}
}

func TestSaveAndLoad(t *testing.T) {
	s := testStore(t)
	sess := testSession("sess-1")

	if err := s.Save(sess); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := s.Load("sess-1")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.ID != sess.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, sess.ID)
	}
	if loaded.WorkflowType != sess.WorkflowType {
		t.Errorf("WorkflowType = %q", loaded.WorkflowType)
	}
	if loaded.IssueNumber != 42 {
		t.Errorf("IssueNumber = %d", loaded.IssueNumber)
	}
	if loaded.AgentName != "claude" {
		t.Errorf("AgentName = %q", loaded.AgentName)
	}
	if loaded.Status != StatusCompleted {
		t.Errorf("Status = %q", loaded.Status)
	}
	if !loaded.StartedAt.Equal(sess.StartedAt) {
		t.Errorf("StartedAt = %v", loaded.StartedAt)
	}
}

func TestLoad_NotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.Load("nonexistent")
	if err == nil {
		t.Error("Load() should error for nonexistent session")
	}
}

func TestList_Empty(t *testing.T) {
	s := testStore(t)
	sessions, err := s.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("List() returned %d sessions, want 0", len(sessions))
	}
}

func TestList_MultipleSessions(t *testing.T) {
	s := testStore(t)

	s1 := testSession("s1")
	s1.StartedAt = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	s2 := testSession("s2")
	s2.StartedAt = time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	s3 := testSession("s3")
	s3.StartedAt = time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)

	// Save in non-chronological order
	s.Save(s2)
	s.Save(s1)
	s.Save(s3)

	sessions, err := s.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("List() returned %d sessions, want 3", len(sessions))
	}

	// Should be sorted newest first
	if sessions[0].ID != "s3" {
		t.Errorf("sessions[0].ID = %q, want s3 (newest)", sessions[0].ID)
	}
	if sessions[1].ID != "s2" {
		t.Errorf("sessions[1].ID = %q, want s2", sessions[1].ID)
	}
	if sessions[2].ID != "s1" {
		t.Errorf("sessions[2].ID = %q, want s1 (oldest)", sessions[2].ID)
	}
}

func TestDelete(t *testing.T) {
	s := testStore(t)
	sess := testSession("del-1")
	s.Save(sess)
	s.AppendOutput("del-1", "some output")

	err := s.Delete("del-1")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err = s.Load("del-1")
	if err == nil {
		t.Error("Load() should fail after Delete()")
	}

	// Output file should also be removed
	if _, statErr := os.Stat(s.OutputPath("del-1")); !os.IsNotExist(statErr) {
		t.Error("output file should be removed after Delete()")
	}
}

func TestDelete_NonexistentIsError(t *testing.T) {
	s := testStore(t)
	err := s.Delete("nonexistent")
	if err == nil {
		t.Error("Delete() should error for nonexistent session")
	}
}

func TestLoadOutput(t *testing.T) {
	s := testStore(t)

	// No file yet — should return empty string, no error
	out, err := s.LoadOutput("no-such-session")
	if err != nil {
		t.Fatalf("LoadOutput() error for missing file: %v", err)
	}
	if out != "" {
		t.Errorf("LoadOutput() = %q, want empty string", out)
	}

	// Write some output then load it back
	s.AppendOutput("load-1", "hello\n")
	s.AppendOutput("load-1", "world\n")

	out, err = s.LoadOutput("load-1")
	if err != nil {
		t.Fatalf("LoadOutput() error: %v", err)
	}
	want := "hello\nworld\n"
	if out != want {
		t.Errorf("LoadOutput() = %q, want %q", out, want)
	}
}

func TestAppendOutput(t *testing.T) {
	s := testStore(t)

	err := s.AppendOutput("out-1", "first line\n")
	if err != nil {
		t.Fatalf("AppendOutput() error: %v", err)
	}
	err = s.AppendOutput("out-1", "second line\n")
	if err != nil {
		t.Fatalf("second AppendOutput() error: %v", err)
	}

	data, err := os.ReadFile(s.OutputPath("out-1"))
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	want := "first line\nsecond line\n"
	if string(data) != want {
		t.Errorf("output = %q, want %q", string(data), want)
	}
}

func TestOutputPath(t *testing.T) {
	s := testStore(t)
	path := s.OutputPath("test-id")
	if filepath.Ext(path) != ".log" {
		t.Errorf("OutputPath extension = %q, want .log", filepath.Ext(path))
	}
}

func TestSave_UpdateExisting(t *testing.T) {
	s := testStore(t)
	sess := testSession("update-1")
	sess.Status = StatusRunning
	s.Save(sess)

	// Update status
	now := time.Now().UTC()
	sess.Status = StatusCompleted
	sess.CompletedAt = &now
	s.Save(sess)

	loaded, err := s.Load("update-1")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Status != StatusCompleted {
		t.Errorf("Status = %q, want completed", loaded.Status)
	}
	if loaded.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestList_SkipsNonJSON(t *testing.T) {
	s := testStore(t)
	sess := testSession("valid")
	s.Save(sess)

	// Write a non-JSON file in the store directory
	os.WriteFile(filepath.Join(s.dir, "notes.txt"), []byte("not json"), 0o644)
	// Write a .log file
	os.WriteFile(filepath.Join(s.dir, "valid.log"), []byte("log output"), 0o644)

	sessions, err := s.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("List() returned %d sessions, want 1 (should skip non-JSON)", len(sessions))
	}
}

func TestNew_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "store", "dir")
	_, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
		t.Error("New() should create the store directory")
	}
}
