package tui

import "github.com/charmbracelet/lipgloss"

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	dimText   = lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			PaddingLeft(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(dimText).
			PaddingLeft(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(dimText).
			PaddingLeft(1).
			PaddingBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(special)

	labelBug = lipgloss.NewStyle().
			Background(lipgloss.Color("#FF6B6B")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	labelPR = lipgloss.NewStyle().
			Background(lipgloss.Color("#4ECDC4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	labelFeature = lipgloss.NewStyle().
			Background(lipgloss.Color("#45B7D1")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1)

	agentOutputStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				PaddingRight(1)

	completedBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#43BF6D")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	failedBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#FF6B6B")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true)

	runningBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#FFD93D")).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Bold(true)
)
