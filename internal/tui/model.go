// Package tui provides an interactive terminal UI for testwarden.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/HumanHorizon/testwarden/internal/report"
)

// State represents the current phase of the TUI.
type State int

const (
	StateAnalysing State = iota
	StateFixing
	StateDone
)

// Model is the bubbletea model for the testwarden TUI.
type Model struct {
	state    State
	spinner  spinner.Model
	viewport viewport.Model
	width    int
	height   int
	ready    bool

	// Progress
	totalIssues   int
	currentIndex  int
	currentAction string

	// Streaming AI
	streaming   bool
	streamBuf   strings.Builder
	streamLabel string
	lastChunk   time.Time

	// Final report
	results []*report.Result
	err     error
}

// New creates a Model with sensible defaults.
func New() *Model {
	s := spinner.New(spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205"))))

	return &Model{
		state:   StateAnalysing,
		spinner: s,
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Messages

// AnalyseStartedMsg signals that analysis has begun.
type AnalyseStartedMsg struct{}

// FixStartedMsg signals that AI fixes have begun.
type FixStartedMsg struct{ TotalIssues int }

// IssueStartedMsg signals the start of fixing an issue.
type IssueStartedMsg struct {
	Index  int
	Total  int
	Action string
	File   string
}

// StreamChunkMsg is a chunk of AI response.
type StreamChunkMsg struct {
	Chunk string
}

// StreamStartedMsg signals start of an AI stream.
type StreamStartedMsg struct {
	Label string
}

// StreamFinishedMsg signals end of an AI stream.
type StreamFinishedMsg struct{}

// DoneMsg carries the final results.
type DoneMsg struct {
	Results []*report.Result
}

// ErrorMsg signals a fatal error.
type ErrorMsg struct{ Err error }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-8)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 8
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case AnalyseStartedMsg:
		m.state = StateAnalysing
		m.currentAction = "Analysing project"
		m.appendLog("→ Analysing project...")
		return m, nil

	case FixStartedMsg:
		m.state = StateFixing
		m.totalIssues = msg.TotalIssues
		m.currentIndex = 0
		m.currentAction = "Fixing"
		m.appendLog(fmt.Sprintf("→ Found %d issue(s) to fix", msg.TotalIssues))
		return m, nil

	case IssueStartedMsg:
		m.currentIndex = msg.Index
		m.totalIssues = msg.Total
		m.currentAction = msg.Action
		m.streaming = false
		m.appendLog(fmt.Sprintf("[%d/%d] %s: %s", msg.Index+1, msg.Total, msg.Action, msg.File))
		return m, nil

	case StreamStartedMsg:
		m.streaming = true
		m.streamBuf.Reset()
		m.streamLabel = msg.Label
		m.lastChunk = time.Now()
		return m, nil

	case StreamChunkMsg:
		m.streamBuf.WriteString(msg.Chunk)
		m.lastChunk = time.Now()
		return m, nil

	case StreamFinishedMsg:
		m.streaming = false
		m.appendLog(fmt.Sprintf("  ✓ %s: generated %d bytes", m.streamLabel, m.streamBuf.Len()))
		return m, nil

	case DoneMsg:
		m.state = StateDone
		m.results = msg.Results
		m.appendLog("→ Done")
		return m, tea.Quit

	case ErrorMsg:
		m.err = msg.Err
		m.appendLog(fmt.Sprintf("✗ Error: %v", msg.Err))
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	m.viewport, _ = m.viewport.Update(msg)
	return m, nil
}

func (m *Model) appendLog(line string) {
	m.viewport.SetContent(m.viewport.View() + "\n" + line)
	m.viewport.GotoBottom()
}

// View implements tea.Model.
func (m *Model) View() string {
	if !m.ready {
		return "Initialising..."
	}

	var sections []string

	// Header
	header := headerStyle.Render("🛡  testwarden")
	sections = append(sections, header)

	// Status
	var status string
	switch m.state {
	case StateAnalysing:
		status = fmt.Sprintf("%s %s", m.spinner.View(), m.currentAction)
	case StateFixing:
		pct := 0.0
		if m.totalIssues > 0 {
			pct = float64(m.currentIndex) / float64(m.totalIssues) * 100
		}
		bar := progressBar(pct, 30)
		status = fmt.Sprintf("%s %s [%d/%d] %s\n%s",
			m.spinner.View(), "Fixing", m.currentIndex+1, m.totalIssues, m.currentAction, bar)
	case StateDone:
		status = doneStyle.Render("✓ Done")
	}
	sections = append(sections, statusStyle.Render(status))

	// Streaming
	if m.streaming {
		preview := m.streamBuf.String()
		if len(preview) > 200 {
			preview = preview[len(preview)-200:]
		}
		stream := streamBoxStyle.Render(
			fmt.Sprintf("%s streaming…\n%s", m.spinner.View(), preview),
		)
		sections = append(sections, stream)
	}

	// Log
	sections = append(sections, logBoxStyle.Render(m.viewport.View()))

	return strings.Join(sections, "\n")
}

// Styles
var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Bold(true).
			Padding(0, 2)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63"))

	streamBoxStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(0, 1)

	logBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	doneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)
)

func progressBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %.0f%%", bar, pct)
}
