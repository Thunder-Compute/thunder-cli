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
	phases       []Phase
	currentPhase int
	spinner      spinner.Model
	startTime    time.Time
	err          error
	quitting     bool
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
				Foreground(lipgloss.Color("#7D56F4")).
				MarginTop(1).
				MarginBottom(1)

	phaseStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D787"))

	inProgressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4"))

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
			BorderForeground(lipgloss.Color("#7D56F4"))
)

func NewConnectFlowModel(instanceID string) ConnectFlowModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	phases := []Phase{
		{Name: "Pre-connection setup", Status: PhasePending},
		{Name: "Instance validation", Status: PhasePending},
		{Name: "SSH key management", Status: PhasePending},
		{Name: "Establishing SSH connection", Status: PhasePending},
		{Name: "Environment setup", Status: PhasePending},
		{Name: "Thunder virtualization", Status: PhasePending},
		{Name: "SSH config update", Status: PhasePending},
		{Name: "Starting SSH session", Status: PhasePending},
	}

	return ConnectFlowModel{
		phases:       phases,
		currentPhase: -1,
		spinner:      s,
		startTime:    time.Now(),
	}
}

func (m ConnectFlowModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m ConnectFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

	case PhaseUpdateMsg:
		if msg.PhaseIndex >= 0 && msg.PhaseIndex < len(m.phases) {
			m.phases[msg.PhaseIndex].Status = msg.Status
			m.phases[msg.PhaseIndex].Message = msg.Message
			m.phases[msg.PhaseIndex].Duration = msg.Duration
			if msg.Status == PhaseInProgress {
				m.currentPhase = msg.PhaseIndex
			}
		}
		return m, nil

	case PhaseCompleteMsg:
		if msg.PhaseIndex >= 0 && msg.PhaseIndex < len(m.phases) {
			m.phases[msg.PhaseIndex].Status = PhaseCompleted
			m.phases[msg.PhaseIndex].Duration = msg.Duration
		}
		return m, nil

	case ConnectCompleteMsg:
		m.quitting = true
		return m, tea.Quit

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

	// Title
	b.WriteString(connectTitleStyle.Render("⚡ Connecting to Thunder Instance"))
	b.WriteString("\n\n")

	// Phases
	for _, phase := range m.phases {
		var icon string
		var style lipgloss.Style
		var line string

		switch phase.Status {
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

	// Summary
	if m.quitting && m.err == nil {
		totalTime := time.Since(m.startTime)
		summary := summaryStyle.Render(
			fmt.Sprintf("✓ Connected successfully in %s", totalTime.Round(time.Millisecond)),
		)
		b.WriteString("\n")
		b.WriteString(summary)
		b.WriteString("\n\n")
	}

	return b.String()
}

// Helper functions to send updates from the main connect flow

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
