package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cmurray/acp-issue-analyzer/internal/store"
	"github.com/cmurray/acp-issue-analyzer/internal/workflow"
)

// completedListModel shows the history of completed agent sessions.
type completedListModel struct {
	table    table.Model
	sessions []*store.PersistedSession
	width    int
	height   int
}

func newCompletedListModel() completedListModel {
	columns := []table.Column{
		{Title: "Repo", Width: 20},
		{Title: "#", Width: 6},
		{Title: "Type", Width: 12},
		{Title: "Agent", Width: 10},
		{Title: "Status", Width: 12},
		{Title: "Date", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(subtle).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7D56F4")).
		Bold(true)
	t.SetStyles(s)

	return completedListModel{table: t}
}

func (m *completedListModel) setSessions(sessions []*store.PersistedSession) {
	m.sessions = sessions
	rows := make([]table.Row, len(sessions))
	for i, s := range sessions {
		rows[i] = table.Row{
			s.Owner + "/" + s.Repo,
			fmt.Sprintf("%d", s.IssueNumber),
			workflow.WorkflowDisplayName(s.WorkflowType),
			s.AgentName,
			string(s.Status),
			s.StartedAt.Local().Format("2006-01-02 15:04"),
		}
	}
	m.table.SetRows(rows)
}

func (m *completedListModel) selectedSession() *store.PersistedSession {
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.sessions) {
		return m.sessions[idx]
	}
	return nil
}

func (m *completedListModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetWidth(w)
	m.table.SetHeight(h - 5)
}

func (m completedListModel) Update(msg tea.Msg) (completedListModel, tea.Cmd) {
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m completedListModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Completed Sessions"))
	b.WriteString("\n")
	if len(m.sessions) == 0 {
		b.WriteString("\n  No completed sessions yet.\n")
	} else {
		b.WriteString(m.table.View())
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter view • tab inbox • q quit"))
	return b.String()
}

// completedDetailModel shows details of a single completed session.
type completedDetailModel struct {
	viewport viewport.Model
	session  *store.PersistedSession
	entries  []store.OutputEntry
	ready    bool
	width    int
	height   int
}

func newCompletedDetailModel() completedDetailModel {
	return completedDetailModel{}
}

func (m *completedDetailModel) setSession(s *store.PersistedSession) {
	m.session = s
	m.entries = nil
	m.updateContent()
}

func (m *completedDetailModel) setEntries(entries []store.OutputEntry) {
	m.entries = entries
	m.updateContent()
}

func (m *completedDetailModel) updateContent() {
	if m.session == nil {
		return
	}
	s := m.session

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Session: %s\n", s.ID))
	b.WriteString(fmt.Sprintf("Repo:    %s/%s\n", s.Owner, s.Repo))
	b.WriteString(fmt.Sprintf("Issue:   #%d — %s\n", s.IssueNumber, s.IssueTitle))
	b.WriteString(fmt.Sprintf("Type:    %s\n", workflow.WorkflowDisplayName(s.WorkflowType)))
	b.WriteString(fmt.Sprintf("Agent:   %s\n", s.AgentName))
	b.WriteString(fmt.Sprintf("Status:  %s\n", s.Status))
	b.WriteString(fmt.Sprintf("Started: %s\n", s.StartedAt.Local().Format("2006-01-02 15:04:05")))
	if s.CompletedAt != nil {
		b.WriteString(fmt.Sprintf("Ended:   %s\n", s.CompletedAt.Local().Format("2006-01-02 15:04:05")))
	}
	b.WriteString(fmt.Sprintf("Worktree: %s\n", s.WorktreePath))
	b.WriteString("\n")

	if s.ResumeCmd != "" {
		b.WriteString("Resume command:\n")
		b.WriteString(fmt.Sprintf("  cd %s && %s\n", s.WorktreePath, s.ResumeCmd))
	}

	if len(m.entries) > 0 {
		b.WriteString("\n── Agent Output ─────────────────────────────────────\n\n")
		b.WriteString(agentOutputStyle.Render(renderEntries(m.entries)))
	}

	m.viewport.SetContent(b.String())
	m.viewport.GotoTop()
}

func (m *completedDetailModel) setSize(w, h int) {
	m.width = w
	m.height = h
	if !m.ready {
		m.viewport = viewport.New(w, h-4)
		m.viewport.YPosition = 2
		m.ready = true
	} else {
		m.viewport.Width = w
		m.viewport.Height = h - 4
	}
	m.updateContent()
}

func (m completedDetailModel) Update(msg tea.Msg) (completedDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m completedDetailModel) View() string {
	if m.session == nil {
		return "No session selected"
	}

	var b strings.Builder
	badge := completedBadge.Render(string(m.session.Status))
	if m.session.Status == store.StatusFailed {
		badge = failedBadge.Render("FAILED")
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Session %s ", m.session.ID)) + badge)
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("esc back"))
	return b.String()
}
