package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	acp "github.com/coder/acp-go-sdk"
)

// Styles for agent output segments.
var (
	thoughtStyle = lipgloss.NewStyle().
			Foreground(dimText).
			Italic(true)

	toolTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#5B44C0", Dark: "#A78BFA"}).
			Bold(true)

	toolStatusStyle = lipgloss.NewStyle().
			Foreground(dimText)

	messageStyle = lipgloss.NewStyle()
)

type agentViewModel struct {
	viewport viewport.Model
	spinner  spinner.Model
	output   strings.Builder
	running  bool
	ready    bool
	width    int
	height   int
	title    string

	// State tracking for clean rendering.
	inThought  bool                       // currently accumulating thought text
	inMessage  bool                       // currently accumulating message text
	toolTitles map[acp.ToolCallId]string   // tool call ID → title
}

func newAgentViewModel() agentViewModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(highlight)
	return agentViewModel{
		spinner:    s,
		running:    true,
		toolTitles: make(map[acp.ToolCallId]string),
	}
}

func (m *agentViewModel) setTitle(title string) {
	m.title = title
}

func (m *agentViewModel) appendUpdate(update acp.SessionUpdate) {
	switch {
	case update.AgentThoughtChunk != nil:
		c := update.AgentThoughtChunk.Content
		if c.Text == nil || c.Text.Text == "" {
			break
		}
		// End any open message block before starting thought.
		if m.inMessage {
			m.output.WriteString("\n")
			m.inMessage = false
		}
		// Start thought header once, then accumulate text.
		if !m.inThought {
			m.output.WriteString(thoughtStyle.Render("  thinking: "))
			m.inThought = true
		}
		m.output.WriteString(thoughtStyle.Render(c.Text.Text))

	case update.AgentMessageChunk != nil:
		c := update.AgentMessageChunk.Content
		if c.Text == nil || c.Text.Text == "" {
			break
		}
		// End any open thought block before starting message.
		if m.inThought {
			m.output.WriteString("\n\n")
			m.inThought = false
		}
		m.inMessage = true
		m.output.WriteString(messageStyle.Render(c.Text.Text))

	case update.ToolCall != nil:
		m.endOpenBlocks()
		tc := update.ToolCall
		m.toolTitles[tc.ToolCallId] = tc.Title

		icon := toolIcon(tc.Kind)
		status := toolStatusStyle.Render(string(tc.Status))
		m.output.WriteString(fmt.Sprintf("\n  %s %s  %s\n", icon, toolTitleStyle.Render(tc.Title), status))

	case update.ToolCallUpdate != nil:
		m.endOpenBlocks()
		tcu := update.ToolCallUpdate
		title := string(tcu.ToolCallId)
		if t, ok := m.toolTitles[tcu.ToolCallId]; ok {
			title = t
		}
		// Update the title mapping if a new title is provided.
		if tcu.Title != nil {
			m.toolTitles[tcu.ToolCallId] = *tcu.Title
			title = *tcu.Title
		}
		status := ""
		if tcu.Status != nil {
			status = string(*tcu.Status)
		}
		if status != "" {
			icon := "  "
			if status == "completed" {
				icon = "  +"
			} else if status == "failed" {
				icon = "  !"
			}
			m.output.WriteString(fmt.Sprintf("%s %s  %s\n", icon, toolTitleStyle.Render(title), toolStatusStyle.Render(status)))
		}

	case update.Plan != nil:
		m.endOpenBlocks()
		m.output.WriteString("\n  Plan:\n")
		for _, entry := range update.Plan.Entries {
			marker := "  "
			switch entry.Status {
			case acp.PlanEntryStatusCompleted:
				marker = "  [x]"
			case acp.PlanEntryStatusInProgress:
				marker = "  [>]"
			default:
				marker = "  [ ]"
			}
			m.output.WriteString(fmt.Sprintf("%s %s\n", marker, entry.Content))
		}
		m.output.WriteString("\n")
	}

	m.viewport.SetContent(agentOutputStyle.Render(m.output.String()))
	m.viewport.GotoBottom()
}

// endOpenBlocks closes any in-progress thought or message block.
func (m *agentViewModel) endOpenBlocks() {
	if m.inThought {
		m.output.WriteString("\n")
		m.inThought = false
	}
	if m.inMessage {
		m.output.WriteString("\n")
		m.inMessage = false
	}
}

func (m *agentViewModel) setDone() {
	m.endOpenBlocks()
	m.running = false
	m.viewport.SetContent(agentOutputStyle.Render(m.output.String()))
}

func (m *agentViewModel) reset() {
	m.output.Reset()
	m.running = true
	m.inThought = false
	m.inMessage = false
	m.toolTitles = make(map[acp.ToolCallId]string)
	m.viewport.SetContent("")
	m.viewport.GotoTop()
}

func (m *agentViewModel) setSize(w, h int) {
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
}

func (m agentViewModel) Update(msg tea.Msg) (agentViewModel, tea.Cmd) {
	var cmds []tea.Cmd

	if m.running {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m agentViewModel) View() string {
	var b strings.Builder

	header := m.title
	if m.running {
		header = m.spinner.View() + " " + header + " (running)"
	} else {
		header = "+ " + header + " (complete)"
	}
	b.WriteString(titleStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	if m.running {
		b.WriteString(helpStyle.Render("esc detach • x cancel • up/down scroll"))
	} else {
		b.WriteString(helpStyle.Render("esc back • up/down scroll"))
	}
	return b.String()
}

// toolIcon returns a short icon string based on tool kind.
func toolIcon(kind acp.ToolKind) string {
	switch kind {
	case acp.ToolKindRead:
		return "R"
	case acp.ToolKindEdit:
		return "E"
	case acp.ToolKindDelete:
		return "D"
	case acp.ToolKindSearch:
		return "?"
	case acp.ToolKindExecute:
		return "$"
	case acp.ToolKindFetch:
		return ">"
	case acp.ToolKindThink:
		return "~"
	default:
		return "*"
	}
}
