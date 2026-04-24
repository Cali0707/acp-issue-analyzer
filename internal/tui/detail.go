package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/Cali0707/baton/internal/github"
)

type detailModel struct {
	viewport viewport.Model
	item     *github.WorkItem
	ready    bool
	width    int
	height   int
}

func newDetailModel() detailModel {
	return detailModel{}
}

func (m *detailModel) setItem(item *github.WorkItem) {
	m.item = item
	m.updateContent()
}

func (m *detailModel) setComments(comments []github.Comment) {
	if m.item != nil {
		m.item.Comments = comments
		m.updateContent()
	}
}

func (m *detailModel) updateContent() {
	if m.item == nil {
		return
	}

	var b strings.Builder
	item := m.item

	kind := "Issue"
	if item.Kind == github.KindPR {
		kind = "Pull Request"
	}

	b.WriteString(fmt.Sprintf("# %s #%d: %s\n\n", kind, item.Number, item.Title))
	b.WriteString(fmt.Sprintf("**Author:** %s  \n", item.Author))
	if len(item.Labels) > 0 {
		b.WriteString(fmt.Sprintf("**Labels:** %s  \n", strings.Join(item.Labels, ", ")))
	}
	b.WriteString("\n---\n\n")
	b.WriteString(item.Body)
	b.WriteString("\n")

	if len(item.Comments) > 0 {
		b.WriteString("\n---\n\n## Comments\n\n")
		for _, c := range item.Comments {
			b.WriteString(fmt.Sprintf("**%s:**\n%s\n\n", c.Author, c.Body))
		}
	}

	rendered, err := glamour.Render(b.String(), "dark")
	if err != nil {
		rendered = b.String()
	}

	m.viewport.SetContent(rendered)
	m.viewport.GotoTop()
}

func (m *detailModel) setSize(w, h int) {
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

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m detailModel) View() string {
	if m.item == nil {
		return "No item selected"
	}

	var b strings.Builder
	kind := "Issue"
	if m.item.Kind == github.KindPR {
		kind = "PR"
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("%s #%d", kind, m.item.Number)))
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("a analyze • esc back • ↑/↓ scroll"))
	return b.String()
}
