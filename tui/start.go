package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type startStep int

const (
	startStepSelect startStep = iota
	startStepConfirm
	startStepComplete
)

type startModel struct {
	step      startStep
	cursor    int
	instances []api.Instance
	selected  *api.Instance
	confirmed bool
	quitting  bool
	client    *api.Client
	loading   bool
	spinner   spinner.Model
	err       error

	styles startStyles
}

type startStyles struct {
	title       lipgloss.Style
	selected    lipgloss.Style
	cursor      lipgloss.Style
	instanceBox lipgloss.Style
	label       lipgloss.Style
	help        lipgloss.Style
}

func newStartStyles() startStyles {
	return startStyles{
		title:    PrimaryTitleStyle().MarginTop(1).MarginBottom(1),
		selected: PrimarySelectedStyle(),
		cursor:   PrimaryCursorStyle(),
		instanceBox: PrimaryStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(theme.PrimaryColor)).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1),
		label: LabelStyle(),
		help:  HelpStyle(),
	}
}

func NewStartModel(client *api.Client, instances []api.Instance) startModel {
	s := NewPrimarySpinner()

	return startModel{
		step:      startStepSelect,
		client:    client,
		loading:   false,
		spinner:   s,
		instances: instances,
		styles:    newStartStyles(),
	}
}

func (m startModel) Init() tea.Cmd {
	return nil
}

func (m startModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.loading {
			switch msg.String() {
			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.step == startStepConfirm {
				m.step = startStepSelect
				m.cursor = 0
			} else {
				m.quitting = true
				return m, tea.Quit
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			maxCursor := m.getMaxCursor()
			if m.cursor < maxCursor {
				m.cursor++
			}
		}
	}

	return m, nil
}

func (m startModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case startStepSelect:
		if m.cursor < len(m.instances) {
			m.selected = &m.instances[m.cursor]
			m.step = startStepConfirm
			m.cursor = 0
		}

	case startStepConfirm:
		if m.cursor == 0 {
			m.confirmed = true
			m.step = startStepComplete
			return m, tea.Quit
		}
		m.step = startStepSelect
		m.cursor = 0
	}

	return m, nil
}

func (m startModel) getMaxCursor() int {
	switch m.step {
	case startStepSelect:
		return len(m.instances) - 1
	case startStepConfirm:
		return 1
	}
	return 0
}

func (m startModel) View() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	if m.quitting {
		return ""
	}

	if m.step == startStepComplete {
		return ""
	}

	var s strings.Builder

	s.WriteString(m.styles.title.Render("⚡ Start Thunder Compute Instance"))
	s.WriteString("\n")

	switch m.step {
	case startStepSelect:
		s.WriteString("Select an instance to start:\n\n")

		for i, instance := range m.instances {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}

			var statusStyle lipgloss.Style
			statusSuffix := ""
			switch instance.Status {
			case "STOPPED":
				statusStyle = WarningStyle()
			case "RUNNING":
				statusStyle = SuccessStyle()
				statusSuffix = " (already running)"
			case "STARTING":
				statusStyle = WarningStyle()
				statusSuffix = " (already starting)"
			default:
				statusStyle = lipgloss.NewStyle()
			}

			idAndName := fmt.Sprintf("(%s) %s", instance.ID, instance.Name)
			if m.cursor == i {
				idAndName = m.styles.selected.Render(idAndName)
			}

			statusText := statusStyle.Render(fmt.Sprintf("(%s)", instance.Status))
			rest := fmt.Sprintf(" %s%s - %sx%s - %s",
				statusText,
				statusSuffix,
				instance.NumGPUs,
				utils.FormatGPUType(instance.GPUType),
				utils.Capitalize(instance.Mode),
			)

			s.WriteString(fmt.Sprintf("%s%s%s\n", cursor, idAndName, rest))
		}

		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  Q: Cancel\n"))

	case startStepConfirm:
		var instanceInfo strings.Builder
		instanceInfo.WriteString(m.styles.label.Render("ID:           ") + m.selected.ID + "\n")
		instanceInfo.WriteString(m.styles.label.Render("Name:         ") + m.selected.Name + "\n")
		instanceInfo.WriteString(m.styles.label.Render("Status:       ") + m.selected.Status + "\n")
		instanceInfo.WriteString(m.styles.label.Render("Mode:         ") + utils.Capitalize(m.selected.Mode) + "\n")
		instanceInfo.WriteString(m.styles.label.Render("GPU:          ") + m.selected.NumGPUs + "x" + utils.FormatGPUType(m.selected.GPUType) + "\n")
		instanceInfo.WriteString(m.styles.label.Render("Template:     ") + utils.Capitalize(m.selected.Template))

		s.WriteString(m.styles.instanceBox.Render(instanceInfo.String()))
		s.WriteString("\n\n")

		s.WriteString("Start this instance?\n\n")

		options := []string{"✓ Yes, Start Instance", "✗ No, Cancel"}
		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}
			if i == 0 {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, SuccessStyle().Render(option)))
			} else {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
			}
		}

		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Confirm  Esc: Back  Q: Cancel\n"))
	}

	return s.String()
}

func RunStartInteractive(client *api.Client, instances []api.Instance) (*api.Instance, error) {
	InitCommonStyles(os.Stdout)
	m := NewStartModel(client, instances)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(startModel)

	if result.err != nil {
		return nil, result.err
	}

	if result.quitting {
		return nil, &CancellationError{}
	}

	if !result.confirmed || result.selected == nil {
		return nil, &CancellationError{}
	}

	return result.selected, nil
}

type startProgressModel struct {
	spinner    spinner.Model
	message    string
	quitting   bool
	success    bool
	successMsg string
	err        error
	client     *api.Client
	instanceID string
}

type startResultMsg struct {
	err error
}

func startInstanceCmd(client *api.Client, instanceID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.StartInstance(instanceID)
		return startResultMsg{err: err}
	}
}

func newStartProgressModel(client *api.Client, instanceID, message string) startProgressModel {
	s := NewPrimarySpinner()
	return startProgressModel{
		spinner:    s,
		message:    message,
		client:     client,
		instanceID: instanceID,
	}
}

func (m startProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, startInstanceCmd(m.client, m.instanceID))
}

func (m startProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.quitting = true
			return m, tea.Quit
		}
		m.success = true
		m.successMsg = fmt.Sprintf("Successfully started Thunder Compute instance %s", m.instanceID)
		m.quitting = true
		return m, tea.Quit
	case tea.KeyMsg:
		m.quitting = true
		return m, tea.Quit
	case tea.QuitMsg:
		m.quitting = true
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m startProgressModel) View() string {
	if m.success {
		return ""
	}
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

func RunStartProgress(client *api.Client, instanceID string) (string, error) {
	InitCommonStyles(os.Stdout)

	m := newStartProgressModel(client, instanceID, fmt.Sprintf("Starting instance %s...", instanceID))
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running start: %w", err)
	}

	result := finalModel.(startProgressModel)
	if result.err != nil {
		return "", result.err
	}

	if result.success {
		return result.successMsg, nil
	}

	return "", nil
}
