package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Cali0707/baton/internal/workflow"
)

type SessionStatus string

const (
	StatusRunning   SessionStatus = "running"
	StatusCompleted SessionStatus = "completed"
	StatusFailed    SessionStatus = "failed"
	StatusCancelled SessionStatus = "cancelled"
)

type PersistedSession struct {
	ID             string             `json:"id"`
	WorkflowType   workflow.WorkflowType `json:"workflow_type"`
	Owner          string             `json:"owner"`
	Repo           string             `json:"repo"`
	IssueNumber    int                `json:"issue_number"`
	IssueTitle     string             `json:"issue_title"`
	AgentName      string             `json:"agent_name"`
	AgentSessionID string             `json:"agent_session_id"`
	WorktreePath   string             `json:"worktree_path"`
	ResumeCmd      string             `json:"resume_cmd,omitempty"`
	Status         SessionStatus      `json:"status"`
	StartedAt      time.Time          `json:"started_at"`
	CompletedAt    *time.Time         `json:"completed_at,omitempty"`
	OutputFile     string             `json:"output_file"`
}

type Store struct {
	dir string
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating store directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) sessionPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// OutputPath returns the path where agent output text should be stored.
func (s *Store) OutputPath(id string) string {
	return filepath.Join(s.dir, id+".log")
}

func (s *Store) Save(session *PersistedSession) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session %s: %w", session.ID, err)
	}
	path := s.sessionPath(session.ID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing session %s: %w", session.ID, err)
	}
	return nil
}

func (s *Store) Load(id string) (*PersistedSession, error) {
	path := s.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading session %s: %w", id, err)
	}
	var session PersistedSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshaling session %s: %w", id, err)
	}
	return &session, nil
}

func (s *Store) List() ([]*PersistedSession, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("reading store directory: %w", err)
	}

	var sessions []*PersistedSession
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-5] // strip .json
		session, err := s.Load(id)
		if err != nil {
			continue // skip corrupted entries
		}
		sessions = append(sessions, session)
	}

	// Sort by start time, newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})

	return sessions, nil
}

func (s *Store) Delete(id string) error {
	os.Remove(s.OutputPath(id))
	return os.Remove(s.sessionPath(id))
}

// OutputEntry is a structured log entry stored as JSONL in the output file.
type OutputEntry struct {
	Type    string      `json:"type"`              // "thought", "message", "tool_call", "tool_update", "plan"
	Text    string      `json:"text,omitempty"`    // thought/message text
	Kind    string      `json:"kind,omitempty"`    // tool kind
	Title   string      `json:"title,omitempty"`   // tool title
	Status  string      `json:"status,omitempty"`  // tool status
	Entries []PlanEntry `json:"entries,omitempty"` // plan entries
}

// PlanEntry represents a single step in a plan.
type PlanEntry struct {
	Status  string `json:"status"`
	Content string `json:"content"`
}

// LoadEntries reads and parses the JSONL output log for a session.
func (s *Store) LoadEntries(id string) ([]OutputEntry, error) {
	data, err := os.ReadFile(s.OutputPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading output for session %s: %w", id, err)
	}
	var entries []OutputEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry OutputEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip corrupted lines
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// AppendEntry appends a structured entry as a JSON line to the session's output log.
func (s *Store) AppendEntry(id string, entry OutputEntry) error {
	f, err := os.OpenFile(s.OutputPath(id), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}
