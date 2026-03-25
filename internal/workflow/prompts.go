package workflow

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type WorkflowType string

const (
	WorkflowBug     WorkflowType = "bug"
	WorkflowFeature WorkflowType = "feature"
	WorkflowPR      WorkflowType = "pr"
)

type PromptData struct {
	Title    string
	Author   string
	Body     string
	Comments []CommentData
	Number   int
	Repo     string
	Diff     string // PR only
}

type CommentData struct {
	Author string
	Body   string
}

var bugTemplate = template.Must(template.New("bug").Parse(`You are investigating a bug report for the {{.Repo}} repository.

## Issue #{{.Number}}: {{.Title}}
**Reported by:** {{.Author}}

### Description
{{.Body}}
{{- if .Comments}}

### Comments
{{- range .Comments}}
**{{.Author}}:** {{.Body}}
{{end}}
{{- end}}

## Your task
1. Investigate this bug by exploring the codebase.
2. Find the root cause of the issue.
3. Identify the relevant files and code paths.
4. If possible, create a minimal reproducer test that demonstrates the bug.
5. Provide a summary of your findings including:
   - Root cause analysis
   - Relevant files and line numbers
   - Suggested fix approach
   - Any related issues or concerns
`))

var featureTemplate = template.Must(template.New("feature").Parse(`You are analyzing a feature request for the {{.Repo}} repository.

## Issue #{{.Number}}: {{.Title}}
**Requested by:** {{.Author}}

### Description
{{.Body}}
{{- if .Comments}}

### Comments
{{- range .Comments}}
**{{.Author}}:** {{.Body}}
{{end}}
{{- end}}

## Your task
1. Assess the validity and scope of this feature request.
2. Explore the codebase to understand existing patterns and architecture.
3. Identify relevant files, interfaces, and extension points.
4. Evaluate implementation complexity and potential impact.
5. Provide a summary including:
   - Feasibility assessment
   - Relevant existing code and patterns
   - Suggested implementation approach
   - Potential risks or concerns
   - Files that would need to be modified
`))

var prTemplate = template.Must(template.New("pr").Parse(`You are reviewing a pull request for the {{.Repo}} repository.

## PR #{{.Number}}: {{.Title}}
**Author:** {{.Author}}

### Description
{{.Body}}
{{- if .Comments}}

### Comments
{{- range .Comments}}
**{{.Author}}:** {{.Body}}
{{end}}
{{- end}}

### Diff
` + "```diff" + `
{{.Diff}}
` + "```" + `

## Your task
1. Review the code changes in this pull request.
2. Examine the diff carefully for correctness, style, and potential issues.
3. Check for:
   - Logic errors or bugs
   - Edge cases not handled
   - Security concerns
   - Performance implications
   - Test coverage
   - Code style consistency
4. Provide a review summary including:
   - Overall assessment (approve / request changes / needs discussion)
   - Specific strengths of the changes
   - Issues that need to be addressed
   - Suggestions for improvement
   - Areas requiring closer human review
`))

func BuildPrompt(wfType WorkflowType, data PromptData) (string, error) {
	var tmpl *template.Template
	switch wfType {
	case WorkflowBug:
		tmpl = bugTemplate
	case WorkflowFeature:
		tmpl = featureTemplate
	case WorkflowPR:
		tmpl = prTemplate
	default:
		return "", fmt.Errorf("unknown workflow type: %s", wfType)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing %s template: %w", wfType, err)
	}
	return buf.String(), nil
}

func WorkflowDisplayName(wfType WorkflowType) string {
	switch wfType {
	case WorkflowBug:
		return "Bug Investigation"
	case WorkflowFeature:
		return "Feature Analysis"
	case WorkflowPR:
		return "PR Review"
	default:
		return strings.ToTitle(string(wfType))
	}
}
