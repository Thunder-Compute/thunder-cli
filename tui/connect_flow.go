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
	cancelled     bool
	done          bool

	styles connectFlowStyles
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
type connectQuitNow struct{}

type connectFlowStyles struct {
	title       lipgloss.Style
	phase       lipgloss.Style
	inProgress  lipgloss.Style
	pending     lipgloss.Style
	skipped     lipgloss.Style
	duration    lipgloss.Style
}

func NewConnectFlowModel(instanceID string) ConnectFlowModel {
	s := NewPrimarySpinner()

	phases := []Phase{
		{Name: "Pre-connection setup", Status: PhasePending},
		{Name: "Instance validation", Status: PhasePending},
		{Name: "SSH key management", Status: PhasePending},
		{Name: "Establishing SSH connection", Status: PhasePending},
		{Name: "Setting up instance", Status: PhasePending},
	}

	styles := connectFlowStyles{
		title:      PrimaryTitleStyle().MarginTop(1).MarginBottom(1),
		phase:      lipgloss.NewStyle().PaddingLeft(2),
		inProgress: PrimaryStyle(),
		pending:    SubtleTextStyle(),
		skipped:    PrimaryStyle().Foreground(lipgloss.Color("#6272A4")),
		duration:   DurationStyle(),
	}

	return ConnectFlowModel{
		phases:       phases,
		currentPhase: -1,
		spinner:      s,
		startTime:    time.Now(),
		lastPhaseIdx: -1,
		styles:       styles,
	}
}

func connectDeferQuit() tea.Cmd {
	return tea.Tick(1*time.Millisecond, func(time.Time) tea.Msg { return connectQuitNow{} })
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
	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "esc", "ctrl+c":
			m.cancelled = true
			m.quitting = true
			return m, connectDeferQuit()
		}
		return m, nil

	case connectQuitNow:
		return m, tea.Quit

	case PhaseUpdateMsg:
		m.setPhase(msg.PhaseIndex, msg.Status, msg.Message, msg.Duration)
		return m, nil

	case PhaseCompleteMsg:
		m.setPhase(msg.PhaseIndex, PhaseCompleted, "", msg.Duration)
		return m, nil

	case ConnectCompleteMsg:
		m.totalDuration = time.Since(m.startTime)
		m.done = true
		m.quitting = true
		return m, connectDeferQuit()

	case ConnectErrorMsg:
		m.err = msg.Err
		m.quitting = true
		return m, connectDeferQuit()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m ConnectFlowModel) View() string {
	var b strings.Builder

	b.WriteString(m.styles.title.Render("⚡ Connecting to Thunder Instance"))
	b.WriteString("\n")

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
			style = successStyle
		case PhaseInProgress:
			icon = m.spinner.View()
			style = m.styles.inProgress
		case PhaseSkipped:
			icon = "○"
			style = m.styles.skipped
		case PhaseWarning:
			icon = "⚠"
			style = warningStyleTUI
		case PhaseError:
			icon = "✗"
			style = errorStyleTUI
		default: // PhasePending
			icon = "○"
			style = m.styles.pending
		}

		line = fmt.Sprintf("%s %s", icon, phase.Name)

		if phase.Duration > 0 {
			line += m.styles.duration.Render(fmt.Sprintf(" (%s)", phase.Duration.Round(time.Millisecond)))
		}

		if phase.Message != "" && status != PhaseInProgress {
			line += "\n  " + style.Render(phase.Message)
		}

		b.WriteString(m.styles.phase.Render(style.Render(line)))
		b.WriteString("\n")
	}

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyleTUI.Render(fmt.Sprintf("✗ Error: Connection failed: %v", m.err)))
		b.WriteString("\n")
	}
	if m.cancelled {
		b.WriteString("\n")
		b.WriteString(warningStyleTUI.Render("⚠ Cancelled"))
		b.WriteString("\n")
	}
	if m.done {
		b.WriteString("\n")
		b.WriteString(successStyle.Render("✓ Connection established successfully"))
		b.WriteString("\n")
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

func (m ConnectFlowModel) Cancelled() bool { return m.cancelled }

func SendConnectError(p *tea.Program, err error) {
	if p != nil {
		p.Send(ConnectErrorMsg{Err: err})
	}
}
