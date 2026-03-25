package workflow

import (
	"strings"
	"testing"
)

func TestBuildPrompt_Bug(t *testing.T) {
	data := PromptData{
		Title:  "Nil pointer in handler",
		Author: "alice",
		Body:   "The server crashes when POST /api/foo is called without a body.",
		Number: 42,
		Repo:   "myorg/myrepo",
		Comments: []CommentData{
			{Author: "bob", Body: "I can reproduce this on v1.2.3."},
		},
	}

	prompt, err := BuildPrompt(WorkflowBug, data)
	if err != nil {
		t.Fatalf("BuildPrompt() error: %v", err)
	}

	checks := []string{
		"investigating a bug",
		"#42",
		"Nil pointer in handler",
		"alice",
		"POST /api/foo",
		"bob",
		"reproduce",
		"root cause",
		"myorg/myrepo",
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt missing %q", c)
		}
	}
}

func TestBuildPrompt_Feature(t *testing.T) {
	data := PromptData{
		Title:  "Add dark mode support",
		Author: "charlie",
		Body:   "It would be great to have a dark theme option.",
		Number: 99,
		Repo:   "org/app",
	}

	prompt, err := BuildPrompt(WorkflowFeature, data)
	if err != nil {
		t.Fatalf("BuildPrompt() error: %v", err)
	}

	checks := []string{
		"feature request",
		"#99",
		"dark mode",
		"charlie",
		"Feasibility",
		"org/app",
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt missing %q", c)
		}
	}
}

func TestBuildPrompt_PR(t *testing.T) {
	data := PromptData{
		Title:  "Fix race condition in cache",
		Author: "dave",
		Body:   "This PR adds a mutex to prevent concurrent map writes.",
		Number: 55,
		Repo:   "org/lib",
		Diff:   "--- a/cache.go\n+++ b/cache.go\n@@ -10,6 +10,7 @@\n+\tmu sync.Mutex",
	}

	prompt, err := BuildPrompt(WorkflowPR, data)
	if err != nil {
		t.Fatalf("BuildPrompt() error: %v", err)
	}

	checks := []string{
		"reviewing a pull request",
		"#55",
		"race condition",
		"dave",
		"mutex",
		"sync.Mutex",
		"approve",
		"org/lib",
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt missing %q", c)
		}
	}
}

func TestBuildPrompt_NoComments(t *testing.T) {
	data := PromptData{
		Title:  "Some issue",
		Author: "user",
		Body:   "Description here.",
		Number: 1,
		Repo:   "o/r",
	}

	prompt, err := BuildPrompt(WorkflowBug, data)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should not contain "Comments" section header when there are none
	if strings.Contains(prompt, "### Comments") {
		t.Error("prompt should not have Comments section when there are no comments")
	}
}

func TestBuildPrompt_WithComments(t *testing.T) {
	data := PromptData{
		Title:  "Issue with comments",
		Author: "user",
		Body:   "Desc.",
		Number: 2,
		Repo:   "o/r",
		Comments: []CommentData{
			{Author: "a", Body: "comment 1"},
			{Author: "b", Body: "comment 2"},
		},
	}

	prompt, err := BuildPrompt(WorkflowBug, data)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(prompt, "comment 1") || !strings.Contains(prompt, "comment 2") {
		t.Error("prompt should include all comments")
	}
}

func TestBuildPrompt_UnknownType(t *testing.T) {
	_, err := BuildPrompt("unknown", PromptData{})
	if err == nil {
		t.Error("expected error for unknown workflow type")
	}
}

func TestWorkflowDisplayName(t *testing.T) {
	tests := []struct {
		wf   WorkflowType
		want string
	}{
		{WorkflowBug, "Bug Investigation"},
		{WorkflowFeature, "Feature Analysis"},
		{WorkflowPR, "PR Review"},
	}
	for _, tt := range tests {
		got := WorkflowDisplayName(tt.wf)
		if got != tt.want {
			t.Errorf("WorkflowDisplayName(%q) = %q, want %q", tt.wf, got, tt.want)
		}
	}
}
