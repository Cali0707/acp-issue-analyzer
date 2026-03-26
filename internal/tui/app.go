package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	acp "github.com/coder/acp-go-sdk"
	"github.com/cmurray/acp-issue-analyzer/internal/agent"
	"github.com/cmurray/acp-issue-analyzer/internal/config"
	ghclient "github.com/cmurray/acp-issue-analyzer/internal/github"
	"github.com/cmurray/acp-issue-analyzer/internal/store"
	"github.com/cmurray/acp-issue-analyzer/internal/workflow"
	"github.com/cmurray/acp-issue-analyzer/internal/worktree"
	"github.com/google/uuid"
)

type viewState int

const (
	viewInbox viewState = iota
	viewDetail
	viewWorkflowSelect
	viewAgentSelect
	viewAgentRunning
	viewCompleted
	viewCompletedList
	viewCompletedDetail
	viewRunningList
)

// activeAgent holds per-run state for a background agent.
type activeAgent struct {
	sessionID string
	session   *store.PersistedSession
	item      *ghclient.WorkItem
	tracker   *agent.SessionTracker
	cancel    context.CancelFunc
	view      agentViewModel
	startedAt time.Time
}

type Model struct {
	// Current view
	state viewState

	// Sub-models
	inbox           inboxModel
	detail          detailModel
	completedList   completedListModel
	completedDetail completedDetailModel

	// Workflow selection state
	workflowOptions []workflow.WorkflowType
	workflowCursor  int

	// Agent selection state
	agentOptions []string
	agentCursor  int

	// Dependencies
	cfg      *config.Config
	ghClient *ghclient.Client
	wtMgr    *worktree.Manager
	store    *store.Store
	logger   *slog.Logger

	// Multi-agent run state
	activeAgents map[string]*activeAgent
	focusedAgent string   // session ID of currently attached agent
	runningOrder []string // ordered session IDs for the running list
	runningCursor int

	// Navigation context
	currentItem *ghclient.WorkItem

	// Dimensions
	width  int
	height int

	// Loading state
	loading bool
	spinner spinner.Model
	errMsg  string
}

func NewModel(cfg *config.Config, ghClient *ghclient.Client, sessionStore *store.Store, logger *slog.Logger) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return Model{
		state:           viewInbox,
		inbox:           newInboxModel(),
		detail:          newDetailModel(),
		completedList:   newCompletedListModel(),
		completedDetail: newCompletedDetailModel(),
		cfg:             cfg,
		ghClient:        ghClient,
		wtMgr:           worktree.NewManager(),
		store:           sessionStore,
		logger:          logger,
		spinner:         s,
		activeAgents:    make(map[string]*activeAgent),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadWorkItems(),
		m.loadCompletedSessions(),
		m.spinner.Tick,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.inbox.setSize(msg.Width, msg.Height)
		m.detail.setSize(msg.Width, msg.Height)
		m.completedList.setSize(msg.Width, msg.Height)
		m.completedDetail.setSize(msg.Width, msg.Height)
		for _, aa := range m.activeAgents {
			aa.view.setSize(msg.Width, msg.Height)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case WorkItemsLoaded:
		m.loading = false
		if msg.Err != nil {
			m.errMsg = msg.Err.Error()
		} else {
			m.inbox.setItems(msg.Items)
			m.errMsg = ""
		}

	case CommentsLoaded:
		if msg.Err != nil {
			m.errMsg = msg.Err.Error()
		} else {
			m.detail.setComments(msg.Comments)
			if m.currentItem != nil && msg.Diff != "" {
				m.currentItem.Diff = msg.Diff
			}
		}

	case AgentUpdateMsg:
		if aa, ok := m.activeAgents[msg.SessionID]; ok {
			aa.view.appendUpdate(msg.Update)
			if entry := makeOutputEntry(msg.Update); entry != nil {
				m.store.AppendEntry(msg.SessionID, *entry)
			}
			// Re-invoke listener to keep streaming
			cmds = append(cmds, m.listenForUpdates(msg.SessionID, aa.tracker))
		}

	case AgentDoneMsg:
		if aa, ok := m.activeAgents[msg.SessionID]; ok {
			aa.view.setDone()
			delete(m.activeAgents, msg.SessionID)
			m.removeFromRunningOrder(msg.SessionID)

			if msg.Err != nil {
				m.errMsg = msg.Err.Error()
			}
			if m.focusedAgent == msg.SessionID {
				// User is watching this agent — show completed detail
				m.focusedAgent = ""
				if msg.Session != nil {
					m.completedDetail.setSession(msg.Session)
					m.state = viewCompleted
					cmds = append(cmds, m.loadSessionOutput(msg.Session.ID))
				} else {
					m.state = viewInbox
				}
			}
			cmds = append(cmds, m.loadCompletedSessions())
		}

	case SessionOutputLoaded:
		if msg.Err != nil {
			m.errMsg = msg.Err.Error()
		} else if m.completedDetail.session != nil && m.completedDetail.session.ID == msg.SessionID {
			m.completedDetail.setEntries(msg.Entries)
		}

	case completedSessionsLoaded:
		m.completedList.setSessions(msg.sessions)

	case ErrorMsg:
		m.errMsg = msg.Err.Error()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Update current sub-model
	switch m.state {
	case viewInbox:
		var cmd tea.Cmd
		m.inbox, cmd = m.inbox.Update(msg)
		cmds = append(cmds, cmd)
	case viewDetail:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		cmds = append(cmds, cmd)
	case viewAgentRunning:
		if aa, ok := m.activeAgents[m.focusedAgent]; ok {
			var cmd tea.Cmd
			aa.view, cmd = aa.view.Update(msg)
			cmds = append(cmds, cmd)
		}
	case viewCompletedList:
		var cmd tea.Cmd
		m.completedList, cmd = m.completedList.Update(msg)
		cmds = append(cmds, cmd)
	case viewCompletedDetail, viewCompleted:
		var cmd tea.Cmd
		m.completedDetail, cmd = m.completedDetail.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case viewInbox:
		switch {
		case msg.String() == "q" || msg.String() == "ctrl+c":
			m.cancelAllAgents()
			return m, tea.Quit
		case msg.String() == "enter":
			if item := m.inbox.selectedItem(); item != nil {
				m.currentItem = item
				m.detail.setItem(item)
				m.state = viewDetail
				return m, m.loadComments(item)
			}
		case msg.String() == "a":
			if item := m.inbox.selectedItem(); item != nil {
				if m.isItemBeingAnalyzed(item) {
					m.errMsg = fmt.Sprintf("#%d is already being analyzed", item.Number)
					return m, nil
				}
				m.currentItem = item
				m.detail.setItem(item)
				return m.startWorkflowSelect(item)
			}
		case msg.String() == "r":
			m.loading = true
			return m, m.loadWorkItems()
		case msg.String() == "tab":
			m.state = viewCompletedList
			return m, nil
		case msg.String() == "s":
			if len(m.activeAgents) > 0 {
				m.runningCursor = 0
				m.state = viewRunningList
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.inbox, cmd = m.inbox.Update(msg)
			return m, cmd
		}

	case viewDetail:
		switch {
		case msg.String() == "esc":
			m.state = viewInbox
			return m, nil
		case msg.String() == "a":
			if m.currentItem != nil {
				if m.isItemBeingAnalyzed(m.currentItem) {
					m.errMsg = fmt.Sprintf("#%d is already being analyzed", m.currentItem.Number)
					return m, nil
				}
				return m.startWorkflowSelect(m.currentItem)
			}
		case msg.String() == "q":
			m.cancelAllAgents()
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.detail, cmd = m.detail.Update(msg)
			return m, cmd
		}

	case viewWorkflowSelect:
		switch {
		case msg.String() == "esc":
			m.state = viewDetail
			return m, nil
		case msg.String() == "j" || msg.String() == "down":
			if m.workflowCursor < len(m.workflowOptions)-1 {
				m.workflowCursor++
			}
		case msg.String() == "k" || msg.String() == "up":
			if m.workflowCursor > 0 {
				m.workflowCursor--
			}
		case msg.String() == "enter":
			wfType := m.workflowOptions[m.workflowCursor]
			return m.startAgentSelect(wfType)
		case msg.String() == "q":
			m.cancelAllAgents()
			return m, tea.Quit
		}

	case viewAgentSelect:
		switch {
		case msg.String() == "esc":
			m.state = viewWorkflowSelect
			return m, nil
		case msg.String() == "j" || msg.String() == "down":
			if m.agentCursor < len(m.agentOptions)-1 {
				m.agentCursor++
			}
		case msg.String() == "k" || msg.String() == "up":
			if m.agentCursor > 0 {
				m.agentCursor--
			}
		case msg.String() == "enter":
			agentName := m.agentOptions[m.agentCursor]
			return m.startAgent(agentName)
		case msg.String() == "q":
			m.cancelAllAgents()
			return m, tea.Quit
		}

	case viewAgentRunning:
		switch {
		case msg.String() == "esc":
			// Detach — agent keeps running in background
			m.focusedAgent = ""
			m.state = viewInbox
			return m, nil
		case msg.String() == "x":
			// Cancel this agent and return to inbox
			if aa, ok := m.activeAgents[m.focusedAgent]; ok {
				aa.cancel()
			}
			m.focusedAgent = ""
			m.state = viewInbox
			return m, nil
		default:
			if aa, ok := m.activeAgents[m.focusedAgent]; ok {
				var cmd tea.Cmd
				aa.view, cmd = aa.view.Update(msg)
				return m, cmd
			}
		}

	case viewRunningList:
		switch {
		case msg.String() == "esc":
			m.state = viewInbox
			return m, nil
		case msg.String() == "j" || msg.String() == "down":
			if m.runningCursor < len(m.runningOrder)-1 {
				m.runningCursor++
			}
		case msg.String() == "k" || msg.String() == "up":
			if m.runningCursor > 0 {
				m.runningCursor--
			}
		case msg.String() == "enter":
			if m.runningCursor < len(m.runningOrder) {
				sid := m.runningOrder[m.runningCursor]
				if _, ok := m.activeAgents[sid]; ok {
					m.focusedAgent = sid
					m.state = viewAgentRunning
				}
			}
			return m, nil
		case msg.String() == "x":
			if m.runningCursor < len(m.runningOrder) {
				sid := m.runningOrder[m.runningCursor]
				if aa, ok := m.activeAgents[sid]; ok {
					aa.cancel()
				}
			}
			return m, nil
		case msg.String() == "q":
			m.cancelAllAgents()
			return m, tea.Quit
		}

	case viewCompleted:
		switch {
		case msg.String() == "esc" || msg.String() == "q":
			m.state = viewInbox
			return m, nil
		default:
			var cmd tea.Cmd
			m.completedDetail, cmd = m.completedDetail.Update(msg)
			return m, cmd
		}

	case viewCompletedList:
		switch {
		case msg.String() == "q" || msg.String() == "ctrl+c":
			m.cancelAllAgents()
			return m, tea.Quit
		case msg.String() == "tab":
			m.state = viewInbox
			return m, nil
		case msg.String() == "enter":
			if s := m.completedList.selectedSession(); s != nil {
				m.completedDetail.setSession(s)
				m.state = viewCompletedDetail
				return m, m.loadSessionOutput(s.ID)
			}
		default:
			var cmd tea.Cmd
			m.completedList, cmd = m.completedList.Update(msg)
			return m, cmd
		}

	case viewCompletedDetail:
		switch {
		case msg.String() == "esc":
			m.state = viewCompletedList
			return m, nil
		case msg.String() == "q":
			m.cancelAllAgents()
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.completedDetail, cmd = m.completedDetail.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.errMsg != "" && m.state == viewInbox {
		return m.renderInboxView() + "\n" + statusBarStyle.Render("Error: "+m.errMsg)
	}

	switch m.state {
	case viewInbox:
		if m.loading {
			return m.spinner.View() + " Loading issues..."
		}
		return m.renderInboxView()

	case viewDetail:
		return m.detail.View()

	case viewWorkflowSelect:
		return m.renderWorkflowSelect()

	case viewAgentSelect:
		return m.renderAgentSelect()

	case viewAgentRunning:
		if aa, ok := m.activeAgents[m.focusedAgent]; ok {
			return aa.view.View()
		}
		return "Agent not found"

	case viewCompleted:
		return m.completedDetail.View()

	case viewCompletedList:
		return m.completedList.View()

	case viewCompletedDetail:
		return m.completedDetail.View()

	case viewRunningList:
		return m.renderRunningList()
	}

	return ""
}

// renderInboxView renders the inbox with a running-agent badge when applicable.
func (m Model) renderInboxView() string {
	base := m.inbox.View()
	if len(m.activeAgents) > 0 {
		badge := runningBadge.Render(fmt.Sprintf(" %d running ", len(m.activeAgents)))
		// Insert badge after the title line
		lines := strings.SplitN(base, "\n", 2)
		if len(lines) == 2 {
			return lines[0] + " " + badge + "\n" + lines[1]
		}
		return base + " " + badge
	}
	return base
}

// renderRunningList shows all active agents.
func (m Model) renderRunningList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Running Agents"))
	b.WriteString("\n\n")

	if len(m.runningOrder) == 0 {
		b.WriteString("  No agents running.\n")
	} else {
		for i, sid := range m.runningOrder {
			aa, ok := m.activeAgents[sid]
			if !ok {
				continue
			}
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.runningCursor {
				cursor = "▸ "
				style = selectedStyle
			}
			elapsed := time.Since(aa.startedAt).Truncate(time.Second)
			label := fmt.Sprintf("%s/%s #%d — %s", aa.item.Owner, aa.item.Repo, aa.item.Number, elapsed)
			b.WriteString(cursor + style.Render(label) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter attach • x cancel • esc back"))
	return b.String()
}

// --- Workflow helpers ---

func (m *Model) startWorkflowSelect(item *ghclient.WorkItem) (tea.Model, tea.Cmd) {
	if item.Kind == ghclient.KindPR {
		// PRs go straight to agent select with PR workflow
		return m.startAgentSelect(workflow.WorkflowPR)
	}
	m.workflowOptions = []workflow.WorkflowType{workflow.WorkflowBug, workflow.WorkflowFeature}
	m.workflowCursor = 0
	m.state = viewWorkflowSelect
	return *m, nil
}

func (m *Model) startAgentSelect(wfType workflow.WorkflowType) (tea.Model, tea.Cmd) {
	m.agentOptions = nil
	repoConfig := m.AgentForRepoItem(m.currentItem)
	defaultAgent := m.cfg.AgentForRepo(repoConfig)

	// Put default first
	if defaultAgent != "" {
		m.agentOptions = append(m.agentOptions, defaultAgent)
	}
	for name := range m.cfg.Agents {
		if name != defaultAgent {
			m.agentOptions = append(m.agentOptions, name)
		}
	}
	m.agentCursor = 0

	// Store workflow type for later
	m.workflowOptions = []workflow.WorkflowType{wfType}

	if len(m.agentOptions) == 1 {
		// Only one agent, skip selection
		return m.startAgent(m.agentOptions[0])
	}

	m.state = viewAgentSelect
	return *m, nil
}

func (m *Model) startAgent(agentName string) (tea.Model, tea.Cmd) {
	item := m.currentItem
	wfType := m.workflowOptions[0]

	sessionID := uuid.New().String()
	tracker := agent.NewSessionTracker()

	ctx, cancel := context.WithCancel(context.Background())

	view := newAgentViewModel()
	view.setTitle(fmt.Sprintf("%s — %s/%s #%d", workflow.WorkflowDisplayName(wfType), item.Owner, item.Repo, item.Number))
	view.setSize(m.width, m.height)

	aa := &activeAgent{
		sessionID: sessionID,
		item:      item,
		tracker:   tracker,
		cancel:    cancel,
		view:      view,
		startedAt: time.Now(),
	}
	m.activeAgents[sessionID] = aa
	m.runningOrder = append(m.runningOrder, sessionID)

	// Return to inbox — agent runs in background
	m.state = viewInbox

	return *m, tea.Batch(
		m.spinner.Tick,
		m.runAgent(ctx, sessionID, agentName, wfType, item, tracker),
		m.listenForUpdates(sessionID, tracker),
	)
}

func (m Model) AgentForRepoItem(item *ghclient.WorkItem) config.RepoConfig {
	for _, r := range m.cfg.Repos {
		if r.Owner == item.Owner && r.Name == item.Repo {
			return r
		}
	}
	return config.RepoConfig{Owner: item.Owner, Name: item.Repo}
}

// --- Duplicate guard ---

func (m Model) isItemBeingAnalyzed(item *ghclient.WorkItem) bool {
	for _, aa := range m.activeAgents {
		if aa.item.Owner == item.Owner && aa.item.Repo == item.Repo && aa.item.Number == item.Number {
			return true
		}
	}
	return false
}

// --- Cancellation ---

func (m *Model) cancelAllAgents() {
	for _, aa := range m.activeAgents {
		aa.cancel()
		if aa.session != nil {
			aa.session.Status = store.StatusCancelled
			now := time.Now().UTC()
			aa.session.CompletedAt = &now
			m.store.Save(aa.session)
		}
	}
}

func (m *Model) removeFromRunningOrder(sessionID string) {
	for i, sid := range m.runningOrder {
		if sid == sessionID {
			m.runningOrder = append(m.runningOrder[:i], m.runningOrder[i+1:]...)
			if m.runningCursor >= len(m.runningOrder) && m.runningCursor > 0 {
				m.runningCursor--
			}
			return
		}
	}
}

// --- Tea commands ---

func (m Model) loadWorkItems() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var allItems []ghclient.WorkItem
		for _, repo := range m.cfg.Repos {
			items, err := m.ghClient.FetchWorkItems(ctx, repo.Owner, repo.Name, repo.Labels)
			if err != nil {
				return WorkItemsLoaded{Err: err}
			}
			allItems = append(allItems, items...)
		}
		return WorkItemsLoaded{Items: allItems}
	}
}

func (m Model) loadComments(item *ghclient.WorkItem) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		comments, err := m.ghClient.FetchComments(ctx, item.Owner, item.Repo, item.Number)
		if err != nil {
			return CommentsLoaded{Err: err}
		}
		var diff string
		if item.Kind == ghclient.KindPR {
			diff, err = m.ghClient.FetchPRDiff(ctx, item.Owner, item.Repo, item.Number)
			if err != nil {
				return CommentsLoaded{Err: fmt.Errorf("fetching PR diff: %w", err)}
			}
		}
		return CommentsLoaded{Comments: comments, Diff: diff}
	}
}

func (m Model) loadCompletedSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.store.List()
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return completedSessionsLoaded{sessions: sessions}
	}
}

func (m Model) loadSessionOutput(sessionID string) tea.Cmd {
	return func() tea.Msg {
		entries, err := m.store.LoadEntries(sessionID)
		return SessionOutputLoaded{SessionID: sessionID, Entries: entries, Err: err}
	}
}

type completedSessionsLoaded struct {
	sessions []*store.PersistedSession
}

func (m Model) runAgent(ctx context.Context, sessionID string, agentName string, wfType workflow.WorkflowType, item *ghclient.WorkItem, tracker *agent.SessionTracker) tea.Cmd {
	return func() tea.Msg {
		agentDef := m.cfg.Agents[agentName]
		repoConfig := m.AgentForRepoItem(item)

		// Create worktree
		var wtPath string
		var err error
		if item.Kind == ghclient.KindPR {
			wtPath, err = m.wtMgr.CreateForPR(repoConfig.Path, item.Number)
		} else {
			wtPath, err = m.wtMgr.CreateForIssue(repoConfig.Path, item.Number)
		}
		if err != nil {
			return AgentDoneMsg{SessionID: sessionID, Err: fmt.Errorf("creating worktree: %w", err)}
		}

		// Build prompt
		promptData := workflow.PromptData{
			Title:  item.Title,
			Author: item.Author,
			Body:   item.Body,
			Number: item.Number,
			Repo:   item.Owner + "/" + item.Repo,
			Diff:   item.Diff,
		}
		for _, c := range item.Comments {
			promptData.Comments = append(promptData.Comments, workflow.CommentData{
				Author: c.Author,
				Body:   c.Body,
			})
		}

		prompt, err := workflow.BuildPrompt(wfType, promptData)
		if err != nil {
			return AgentDoneMsg{SessionID: sessionID, Err: fmt.Errorf("building prompt: %w", err)}
		}

		// Create session record
		now := time.Now().UTC()
		sess := &store.PersistedSession{
			ID:           sessionID,
			WorkflowType: wfType,
			Owner:        item.Owner,
			Repo:         item.Repo,
			IssueNumber:  item.Number,
			IssueTitle:   item.Title,
			AgentName:    agentName,
			WorktreePath: wtPath,
			Status:       store.StatusRunning,
			StartedAt:    now,
			OutputFile:   m.store.OutputPath(sessionID),
		}
		m.store.Save(sess)

		// Run the agent
		runner := agent.NewRunner(agentDef, m.cfg.Safety, m.logger)
		result, err := runner.Run(ctx, wtPath, prompt, tracker)

		completedAt := time.Now().UTC()
		sess.CompletedAt = &completedAt

		if err != nil {
			sess.Status = store.StatusFailed
			m.store.Save(sess)
			return AgentDoneMsg{SessionID: sessionID, Session: sess, Err: err}
		}

		sess.Status = store.StatusCompleted
		sess.AgentSessionID = result.SessionID
		if agentDef.ResumeCmd != "" {
			sess.ResumeCmd = agentDef.BuildResumeCmd(result.SessionID)
		}
		m.store.Save(sess)

		tracker.Close()
		return AgentDoneMsg{SessionID: sessionID, Session: sess}
	}
}

func (m Model) listenForUpdates(sessionID string, tracker *agent.SessionTracker) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-tracker.UpdateChan()
		if !ok {
			return nil
		}
		return AgentUpdateMsg{SessionID: sessionID, Update: update}
	}
}

// --- Selector views ---

func (m Model) renderWorkflowSelect() string {
	var b fmt.Stringer = &workflowSelectView{
		options: m.workflowOptions,
		cursor:  m.workflowCursor,
		item:    m.currentItem,
	}
	return b.String()
}

func (m Model) renderAgentSelect() string {
	var b fmt.Stringer = &agentSelectView{
		options: m.agentOptions,
		cursor:  m.agentCursor,
	}
	return b.String()
}

type workflowSelectView struct {
	options []workflow.WorkflowType
	cursor  int
	item    *ghclient.WorkItem
}

func (v *workflowSelectView) String() string {
	var b string
	b += titleStyle.Render(fmt.Sprintf("Analyze #%d: %s", v.item.Number, v.item.Title))
	b += "\n\n"
	b += "  Select workflow type:\n\n"
	for i, opt := range v.options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == v.cursor {
			cursor = "▸ "
			style = selectedStyle
		}
		b += cursor + style.Render(workflow.WorkflowDisplayName(opt)) + "\n"
	}
	b += "\n"
	b += helpStyle.Render("j/k navigate • enter select • esc back")
	return b
}

type agentSelectView struct {
	options []string
	cursor  int
}

func (v *agentSelectView) String() string {
	var b string
	b += titleStyle.Render("Select Agent")
	b += "\n\n"
	for i, opt := range v.options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == v.cursor {
			cursor = "▸ "
			style = selectedStyle
		}
		label := opt
		if i == 0 {
			label += " (default)"
		}
		b += cursor + style.Render(label) + "\n"
	}
	b += "\n"
	b += helpStyle.Render("j/k navigate • enter select • esc back")
	return b
}

func makeOutputEntry(u acp.SessionUpdate) *store.OutputEntry {
	switch {
	case u.AgentMessageChunk != nil && u.AgentMessageChunk.Content.Text != nil:
		return &store.OutputEntry{Type: "message", Text: u.AgentMessageChunk.Content.Text.Text}
	case u.AgentThoughtChunk != nil && u.AgentThoughtChunk.Content.Text != nil:
		return &store.OutputEntry{Type: "thought", Text: u.AgentThoughtChunk.Content.Text.Text}
	case u.ToolCall != nil:
		return &store.OutputEntry{
			Type:   "tool_call",
			Kind:   string(u.ToolCall.Kind),
			Title:  u.ToolCall.Title,
			Status: string(u.ToolCall.Status),
		}
	case u.ToolCallUpdate != nil:
		status := ""
		if u.ToolCallUpdate.Status != nil {
			status = string(*u.ToolCallUpdate.Status)
		}
		title := string(u.ToolCallUpdate.ToolCallId)
		if u.ToolCallUpdate.Title != nil {
			title = *u.ToolCallUpdate.Title
		}
		if status != "" {
			return &store.OutputEntry{Type: "tool_update", Title: title, Status: status}
		}
	case u.Plan != nil:
		entries := make([]store.PlanEntry, len(u.Plan.Entries))
		for i, e := range u.Plan.Entries {
			entries[i] = store.PlanEntry{Status: string(e.Status), Content: e.Content}
		}
		return &store.OutputEntry{Type: "plan", Entries: entries}
	}
	return nil
}
