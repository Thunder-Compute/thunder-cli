package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PhaseStatus int

const (
	PhasePending PhaseStatus = iota
	PhaseInProgress
	PhaseCompleted
	PhaseSkipped
	PhaseWarning
	PhaseError
)

type Phase struct {
	Name     string
	Status   PhaseStatus
	Message  string
	Duration time.Duration
}

type ConnectFlowModel struct {
	phases        []Phase
	currentPhase  int
	spinner       spinner.Model
	startTime     time.Time
	totalDuration time.Duration
	err           error
	quitting      bool
	lastPhaseIdx  int
	awaitingEnter bool
}

type PhaseUpdateMsg struct {
	PhaseIndex int
	Status     PhaseStatus
	Message    string
	Duration   time.Duration
}

type PhaseCompleteMsg struct {
	PhaseIndex int
	Duration   time.Duration
}

type ConnectCompleteMsg struct{}
type ConnectErrorMsg struct{ Err error }

var (
	connectTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#0391ff")).
				MarginTop(1).
				MarginBottom(1)

	phaseStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D787"))

	inProgressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0391ff"))

	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	connectWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFB86C"))

	connectErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF5555"))

	skippedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))

	durationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	summaryStyle = lipgloss.NewStyle().
			MarginTop(1).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#0391ff"))
)

func NewConnectFlowModel(instanceID string) ConnectFlowModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0391ff"))

	phases := []Phase{
		{Name: "Pre-connection setup", Status: PhasePending},
		{Name: "Instance validation", Status: PhasePending},
		{Name: "SSH key management", Status: PhasePending},
		{Name: "Establishing SSH connection", Status: PhasePending},
		{Name: "Setting up instance", Status: PhasePending},
	}

	return ConnectFlowModel{
		phases:       phases,
		currentPhase: -1,
		spinner:      s,
		startTime:    time.Now(),
		lastPhaseIdx: -1,
	}
}

func (m ConnectFlowModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *ConnectFlowModel) setPhase(idx int, status PhaseStatus, msg string, dur time.Duration) {
	if idx < 0 || idx >= len(m.phases) {
		return
	}
	ph := &m.phases[idx]

	if status == PhaseInProgress && ph.Status == PhaseInProgress {
		if msg == "" || msg == ph.Message {
			return
		}
	}

	if ph.Status == status && ph.Message == msg && (dur == 0 || ph.Duration == dur) {
		return
	}

	ph.Status = status
	ph.Message = msg
	if dur > 0 {
		ph.Duration = dur
	}
	if status == PhaseInProgress {
		m.currentPhase = idx
		m.lastPhaseIdx = idx
	}
}

func (m ConnectFlowModel) CurrentPhase() int {
	return m.currentPhase
}

func (m ConnectFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if m.awaitingEnter {
				m.quitting = true
				return m, tea.Quit
			}
		}
		return m, nil

	case PhaseUpdateMsg:
		m.setPhase(msg.PhaseIndex, msg.Status, msg.Message, msg.Duration)
		return m, nil

	case PhaseCompleteMsg:
		m.setPhase(msg.PhaseIndex, PhaseCompleted, "", msg.Duration)
		return m, nil

	case ConnectCompleteMsg:
		m.totalDuration = time.Since(m.startTime)
		m.awaitingEnter = true
		return m, nil

	case ConnectErrorMsg:
		m.err = msg.Err
		m.quitting = true
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m ConnectFlowModel) View() string {
	if m.quitting && m.err != nil {
		return connectErrorStyle.Render(fmt.Sprintf("\n✗ Connection failed: %v\n\n", m.err))
	}

	var b strings.Builder

	b.WriteString(connectTitleStyle.Render("⚡ Connecting to Thunder Instance"))
	b.WriteString("\n\n")

	for i, phase := range m.phases {
		var icon string
		var style lipgloss.Style
		var line string

		status := phase.Status
		if status == PhaseInProgress && i != m.currentPhase {
			status = PhasePending
		}

		switch status {
		case PhaseCompleted:
			icon = "✓"
			style = completedStyle
		case PhaseInProgress:
			icon = m.spinner.View()
			style = inProgressStyle
		case PhaseSkipped:
			icon = "○"
			style = skippedStyle
		case PhaseWarning:
			icon = "⚠"
			style = connectWarningStyle
		case PhaseError:
			icon = "✗"
			style = connectErrorStyle
		default: // PhasePending
			icon = "○"
			style = pendingStyle
		}

		line = fmt.Sprintf("%s %s", icon, phase.Name)

		if phase.Duration > 0 {
			line += durationStyle.Render(fmt.Sprintf(" (%s)", phase.Duration.Round(time.Millisecond)))
		}

		if phase.Message != "" {
			line += "\n  " + style.Render(phase.Message)
		}

		b.WriteString(phaseStyle.Render(style.Render(line)))
		b.WriteString("\n")
	}

	if m.awaitingEnter && m.err == nil {
		summary := summaryStyle.Render(
			fmt.Sprintf("✓ Connected successfully in %s", m.totalDuration.Round(time.Millisecond)),
		)
		b.WriteString("\n")
		b.WriteString(summary)
		b.WriteString("\n\n")
		b.WriteString(pendingStyle.Render("Press Enter to start SSH session..."))
		b.WriteString("\n")
	}

	return b.String()
}

func SendPhaseUpdate(p *tea.Program, phaseIndex int, status PhaseStatus, message string, duration time.Duration) {
	if p != nil {
		p.Send(PhaseUpdateMsg{
			PhaseIndex: phaseIndex,
			Status:     status,
			Message:    message,
			Duration:   duration,
		})
	}
}

func SendPhaseComplete(p *tea.Program, phaseIndex int, duration time.Duration) {
	if p != nil {
		p.Send(PhaseCompleteMsg{
			PhaseIndex: phaseIndex,
			Duration:   duration,
		})
	}
}

func SendConnectComplete(p *tea.Program) {
	if p != nil {
		p.Send(ConnectCompleteMsg{})
	}
}

func SendConnectError(p *tea.Program, err error) {
	if p != nil {
		p.Send(ConnectErrorMsg{Err: err})
	}
}
