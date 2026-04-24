package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Cali0707/baton/internal/github"
)

type inboxModel struct {
	table table.Model
	items []github.WorkItem
	width int
	height int
}

func newInboxModel() inboxModel {
	columns := []table.Column{
		{Title: "Type", Width: 6},
		{Title: "Repo", Width: 20},
		{Title: "#", Width: 6},
		{Title: "Title", Width: 40},
		{Title: "Author", Width: 15},
		{Title: "Updated", Width: 12},
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

	return inboxModel{table: t}
}

func (m *inboxModel) setItems(items []github.WorkItem) {
	m.items = items
	rows := make([]table.Row, len(items))
	for i, item := range items {
		kind := "ISSUE"
		if item.Kind == github.KindPR {
			kind = "PR"
		}
		rows[i] = table.Row{
			kind,
			item.Owner + "/" + item.Repo,
			fmt.Sprintf("%d", item.Number),
			truncate(item.Title, 38),
			item.Author,
			relativeTime(item.UpdatedAt),
		}
	}
	m.table.SetRows(rows)
}

func (m *inboxModel) selectedItem() *github.WorkItem {
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.items) {
		return &m.items[idx]
	}
	return nil
}

func (m inboxModel) Update(msg tea.Msg) (inboxModel, tea.Cmd) {
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m inboxModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Inbox"))
	b.WriteString("\n")
	b.WriteString(m.table.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k navigate • enter view • a analyze • s running • r refresh • tab history • q quit"))
	return b.String()
}

func (m *inboxModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetWidth(w)
	m.table.SetHeight(h - 5) // account for title + help
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
