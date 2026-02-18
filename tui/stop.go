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

type stopStep int

const (
	stopStepSelect stopStep = iota
	stopStepConfirm
	stopStepComplete
)

type stopModel struct {
	step      stopStep
	cursor    int
	instances []api.Instance
	selected  *api.Instance
	confirmed bool
	quitting  bool
	client    *api.Client
	loading   bool
	spinner   spinner.Model
	err       error

	styles stopStyles
}

type stopStyles struct {
	title       lipgloss.Style
	selected    lipgloss.Style
	cursor      lipgloss.Style
	warningBox  lipgloss.Style
	instanceBox lipgloss.Style
	label       lipgloss.Style
	help        lipgloss.Style
}

func newStopStyles() stopStyles {
	return stopStyles{
		title:      PrimaryTitleStyle().MarginTop(1).MarginBottom(1),
		selected:   PrimarySelectedStyle(),
		cursor:     PrimaryCursorStyle(),
		warningBox: WarningBoxStyle().MarginTop(1).MarginBottom(1),
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

func NewStopModel(client *api.Client, instances []api.Instance) stopModel {
	s := NewPrimarySpinner()

	return stopModel{
		step:      stopStepSelect,
		client:    client,
		loading:   false,
		spinner:   s,
		instances: instances,
		styles:    newStopStyles(),
	}
}

func (m stopModel) Init() tea.Cmd {
	return nil
}

func (m stopModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.step == stopStepConfirm {
				m.step = stopStepSelect
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

func (m stopModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stopStepSelect:
		if m.cursor < len(m.instances) {
			m.selected = &m.instances[m.cursor]
			m.step = stopStepConfirm
			m.cursor = 0
		}

	case stopStepConfirm:
		if m.cursor == 0 {
			m.confirmed = true
			m.step = stopStepComplete
			return m, tea.Quit
		}
		m.step = stopStepSelect
		m.cursor = 0
	}

	return m, nil
}

func (m stopModel) getMaxCursor() int {
	switch m.step {
	case stopStepSelect:
		return len(m.instances) - 1
	case stopStepConfirm:
		return 1
	}
	return 0
}

func (m stopModel) View() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	if m.quitting {
		return ""
	}

	if m.step == stopStepComplete {
		return ""
	}

	var s strings.Builder

	s.WriteString(m.styles.title.Render("⚡ Stop Thunder Compute Instance"))
	s.WriteString("\n")

	switch m.step {
	case stopStepSelect:
		s.WriteString("Select an instance to stop:\n\n")

		for i, instance := range m.instances {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}

			var statusStyle lipgloss.Style
			statusSuffix := ""
			switch instance.Status {
			case "RUNNING":
				statusStyle = SuccessStyle()
			case "STOPPING":
				statusStyle = WarningStyle()
				statusSuffix = " (already stopping)"
			case "STOPPED":
				statusStyle = ErrorStyle()
				statusSuffix = " (already stopped)"
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

	case stopStepConfirm:
		var instanceInfo strings.Builder
		instanceInfo.WriteString(m.styles.label.Render("ID:           ") + m.selected.ID + "\n")
		instanceInfo.WriteString(m.styles.label.Render("Name:         ") + m.selected.Name + "\n")
		instanceInfo.WriteString(m.styles.label.Render("Status:       ") + m.selected.Status + "\n")
		instanceInfo.WriteString(m.styles.label.Render("Mode:         ") + utils.Capitalize(m.selected.Mode) + "\n")
		instanceInfo.WriteString(m.styles.label.Render("GPU:          ") + m.selected.NumGPUs + "x" + utils.FormatGPUType(m.selected.GPUType) + "\n")
		instanceInfo.WriteString(m.styles.label.Render("Template:     ") + utils.Capitalize(m.selected.Template))

		s.WriteString(m.styles.instanceBox.Render(instanceInfo.String()))
		s.WriteString("\n\n")

		s.WriteString("Are you sure you want to stop this instance?\n\n")

		options := []string{"✓ Yes, Stop Instance", "✗ No, Cancel"}
		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}
			if i == 0 {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, WarningStyle().Render(option)))
			} else {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
			}
		}

		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Confirm  Esc: Back  Q: Cancel\n"))
	}

	return s.String()
}

func RunStopInteractive(client *api.Client, instances []api.Instance) (*api.Instance, error) {
	InitCommonStyles(os.Stdout)
	m := NewStopModel(client, instances)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(stopModel)

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

type stopProgressModel struct {
	spinner    spinner.Model
	message    string
	quitting   bool
	success    bool
	successMsg string
	err        error
	client     *api.Client
	instanceID string
}

type stopResultMsg struct {
	err error
}

func stopInstanceCmd(client *api.Client, instanceID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.StopInstance(instanceID)
		return stopResultMsg{err: err}
	}
}

func newStopProgressModel(client *api.Client, instanceID, message string) stopProgressModel {
	s := NewPrimarySpinner()
	return stopProgressModel{
		spinner:    s,
		message:    message,
		client:     client,
		instanceID: instanceID,
	}
}

func (m stopProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, stopInstanceCmd(m.client, m.instanceID))
}

func (m stopProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stopResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.quitting = true
			return m, tea.Quit
		}
		m.success = true
		m.successMsg = fmt.Sprintf("Successfully stopped Thunder Compute instance %s", m.instanceID)
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

func (m stopProgressModel) View() string {
	if m.success {
		return ""
	}
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

func RunStopProgress(client *api.Client, instanceID string) (string, error) {
	InitCommonStyles(os.Stdout)

	m := newStopProgressModel(client, instanceID, fmt.Sprintf("Stopping instance %s...", instanceID))
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running stop: %w", err)
	}

	result := finalModel.(stopProgressModel)
	if result.err != nil {
		return "", result.err
	}

	if result.success {
		return result.successMsg, nil
	}

	return "", nil
}
