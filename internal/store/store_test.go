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
	s.AppendEntry("del-1", OutputEntry{Type: "message", Text: "some output"})

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

func TestLoadEntries(t *testing.T) {
	s := testStore(t)

	// No file yet — should return nil, no error
	entries, err := s.LoadEntries("no-such-session")
	if err != nil {
		t.Fatalf("LoadEntries() error for missing file: %v", err)
	}
	if entries != nil {
		t.Errorf("LoadEntries() = %v, want nil", entries)
	}

	// Write some entries then load them back
	s.AppendEntry("load-1", OutputEntry{Type: "thought", Text: "analyzing"})
	s.AppendEntry("load-1", OutputEntry{Type: "message", Text: "hello world"})

	entries, err = s.LoadEntries("load-1")
	if err != nil {
		t.Fatalf("LoadEntries() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("LoadEntries() returned %d entries, want 2", len(entries))
	}
	if entries[0].Type != "thought" || entries[0].Text != "analyzing" {
		t.Errorf("entries[0] = %+v, want thought/analyzing", entries[0])
	}
	if entries[1].Type != "message" || entries[1].Text != "hello world" {
		t.Errorf("entries[1] = %+v, want message/hello world", entries[1])
	}
}

func TestAppendEntry(t *testing.T) {
	s := testStore(t)

	err := s.AppendEntry("out-1", OutputEntry{Type: "tool_call", Kind: "read", Title: "File Reader", Status: "running"})
	if err != nil {
		t.Fatalf("AppendEntry() error: %v", err)
	}
	err = s.AppendEntry("out-1", OutputEntry{Type: "tool_update", Title: "File Reader", Status: "completed"})
	if err != nil {
		t.Fatalf("second AppendEntry() error: %v", err)
	}

	entries, err := s.LoadEntries("out-1")
	if err != nil {
		t.Fatalf("LoadEntries() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Kind != "read" {
		t.Errorf("entries[0].Kind = %q, want read", entries[0].Kind)
	}
	if entries[1].Status != "completed" {
		t.Errorf("entries[1].Status = %q, want completed", entries[1].Status)
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
