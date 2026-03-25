package tui

import (
	acp "github.com/coder/acp-go-sdk"
	"github.com/cmurray/acp-issue-analyzer/internal/github"
	"github.com/cmurray/acp-issue-analyzer/internal/store"
)

// Messages for Bubble Tea

// WorkItemsLoaded is sent when GitHub work items finish loading.
type WorkItemsLoaded struct {
	Items []github.WorkItem
	Err   error
}

// CommentsLoaded is sent when comments for a work item finish loading.
type CommentsLoaded struct {
	Comments []github.Comment
	Diff     string // non-empty for PRs
	Err      error
}

// AgentUpdateMsg wraps a session update from the ACP agent.
type AgentUpdateMsg struct {
	SessionID string
	Update    acp.SessionUpdate
}

// AgentDoneMsg signals the agent workflow is complete.
type AgentDoneMsg struct {
	SessionID string
	Session   *store.PersistedSession
	Err       error
}

// AgentStartedMsg signals the agent started successfully.
type AgentStartedMsg struct {
	SessionID string
}

// SessionOutputLoaded carries the full output text for a completed session.
type SessionOutputLoaded struct {
	SessionID string
	Output    string
	Err       error
}

// ErrorMsg represents a user-facing error.
type ErrorMsg struct {
	Err error
}
