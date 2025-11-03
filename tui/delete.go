package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type deleteStep int

const (
	deleteStepSelect deleteStep = iota
	deleteStepConfirm
	deleteStepComplete
)

type deleteModel struct {
	step      deleteStep
	cursor    int
	instances []api.Instance
	selected  *api.Instance
	confirmed bool
	quitting  bool
	client    *api.Client
	loading   bool
	spinner   spinner.Model
	err       error
}

var (
	deleteTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#0391ff")).
				MarginTop(1).
				MarginBottom(1)

	deleteSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0391ff")).
				Bold(true)

	deleteCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0391ff"))

	deleteWarningStyle = warningStyleTUI.Copy().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FFB86C")).
				Padding(1, 2).
				MarginTop(1).
				MarginBottom(1)

	deleteInstanceStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#0391ff")).
				Padding(1, 2).
				MarginTop(1).
				MarginBottom(1)

	deleteLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF"))
)

func NewDeleteModel(client *api.Client, instances []api.Instance) deleteModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0391ff"))

	return deleteModel{
		step:      deleteStepSelect,
		client:    client,
		loading:   false,
		spinner:   s,
		instances: instances,
	}
}

func (m deleteModel) Init() tea.Cmd {
	return nil
}

func (m deleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// Don't process keys while loading
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
			if m.step == deleteStepConfirm {
				m.step = deleteStepSelect
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

func (m deleteModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case deleteStepSelect:
		if m.cursor < len(m.instances) {
			m.selected = &m.instances[m.cursor]
			m.step = deleteStepConfirm
			m.cursor = 0
		}

	case deleteStepConfirm:
		if m.cursor == 0 {
			m.confirmed = true
			m.step = deleteStepComplete
			return m, tea.Quit
		} else {
			m.step = deleteStepSelect
			m.cursor = 0
		}
	}

	return m, nil
}

func (m deleteModel) getMaxCursor() int {
	switch m.step {
	case deleteStepSelect:
		return len(m.instances) - 1
	case deleteStepConfirm:
		return 1 // Yes/No options
	}
	return 0
}

func (m deleteModel) View() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	if m.quitting {
		return ""
	}

	if m.step == deleteStepComplete {
		return ""
	}

	var s strings.Builder

	s.WriteString(deleteTitleStyle.Render("⚡ Delete Thunder Compute Instance"))
	s.WriteString("\n")

	switch m.step {
	case deleteStepSelect:
		s.WriteString("Select an instance to delete:\n\n")

		for i, instance := range m.instances {
			cursor := "  "
			if m.cursor == i {
				cursor = deleteCursorStyle.Render("▶ ")
			}

			// Determine status style
			var statusStyle lipgloss.Style
			statusSuffix := ""
			switch instance.Status {
			case "RUNNING":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D787")).Bold(true) // Green (success color)
			case "STARTING":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C")).Bold(true) // Yellow (warning color)
			case "DELETING":
				statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true) // Red (error color)
				statusSuffix = " (already deleting)"
			default:
				statusStyle = lipgloss.NewStyle()
			}

			idAndName := fmt.Sprintf("(%s) %s", instance.ID, instance.Name)
			if m.cursor == i {
				idAndName = deleteSelectedStyle.Render(idAndName)
			}

			statusText := statusStyle.Render(fmt.Sprintf("(%s)", instance.Status))
			rest := fmt.Sprintf(" %s%s - %s - %sx%s - %s",
				statusText,
				statusSuffix,
				instance.IP,
				instance.NumGPUs,
				instance.GPUType,
				utils.Capitalize(instance.Mode),
			)

			s.WriteString(fmt.Sprintf("%s%s%s\n", cursor, idAndName, rest))
		}

		s.WriteString("\n")
		s.WriteString(helpStyleTUI.Render("↑/↓: Navigate  Enter: Select  Q: Cancel\n"))

	case deleteStepConfirm:
		warning := "WARNING: This action is IRREVERSIBLE!\n\n" +
			"Deleting this instance will:\n" +
			"• Permanently destroy the instance and ALL data\n" +
			"• Remove all SSH configuration for this instance\n" +
			"• This action CANNOT be undone"
		s.WriteString(deleteWarningStyle.Render(warning))
		s.WriteString("\n\n")

		var instanceInfo strings.Builder
		instanceInfo.WriteString(deleteLabelStyle.Render("ID:           ") + m.selected.ID + "\n")
		instanceInfo.WriteString(deleteLabelStyle.Render("Name:         ") + m.selected.Name + "\n")
		instanceInfo.WriteString(deleteLabelStyle.Render("Status:       ") + m.selected.Status + "\n")
		instanceInfo.WriteString(deleteLabelStyle.Render("IP Address:   ") + m.selected.IP + "\n")
		instanceInfo.WriteString(deleteLabelStyle.Render("Mode:         ") + utils.Capitalize(m.selected.Mode) + "\n")
		instanceInfo.WriteString(deleteLabelStyle.Render("GPU:          ") + m.selected.NumGPUs + "x" + m.selected.GPUType + "\n")
		instanceInfo.WriteString(deleteLabelStyle.Render("Template:     ") + utils.Capitalize(m.selected.Template))

		s.WriteString(deleteInstanceStyle.Render(instanceInfo.String()))
		s.WriteString("\n\n")

		s.WriteString("Are you sure you want to delete this instance?\n\n")

		options := []string{"✓ Yes, Delete Instance", "✗ No, Cancel"}
		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = deleteCursorStyle.Render("▶ ")
			}
			if i == 0 {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true).Render(option)))
			} else {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
			}
		}

		s.WriteString("\n")
		s.WriteString(helpStyleTUI.Render("↑/↓: Navigate  Enter: Confirm  Esc: Back  Q: Cancel\n"))
	}

	return s.String()
}

func RunDeleteInteractive(client *api.Client, instances []api.Instance) (*api.Instance, error) {
	InitCommonStyles(os.Stdout)
	m := NewDeleteModel(client, instances)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(deleteModel)

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

type deleteProgressModel struct {
	spinner    spinner.Model
	message    string
	quitting   bool
	success    bool
	successMsg string
	err        error
	client     *api.Client
	instanceID string
}

type deleteResultMsg struct {
	err error
}

func deleteInstanceCmd(client *api.Client, instanceID string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.DeleteInstance(instanceID)
		return deleteResultMsg{err: err}
	}
}

func newDeleteProgressModel(client *api.Client, instanceID, message string) deleteProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0391ff"))
	return deleteProgressModel{
		spinner:    s,
		message:    message,
		client:     client,
		instanceID: instanceID,
	}
}

func (m deleteProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, deleteInstanceCmd(m.client, m.instanceID))
}

func (m deleteProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case deleteResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.quitting = true
			return m, tea.Quit
		}
		m.success = true
		m.successMsg = fmt.Sprintf("Successfully deleted Thunder Compute instance %s", m.instanceID)
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

func (m deleteProgressModel) View() string {
	if m.success {
		return ""
	}
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

func RunDeleteProgress(client *api.Client, instanceID string) (string, error) {
	InitCommonStyles(os.Stdout)

	m := newDeleteProgressModel(client, instanceID, fmt.Sprintf("Deleting instance %s...", instanceID))
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running deletion: %w", err)
	}

	result := finalModel.(deleteProgressModel)
	if result.err != nil {
		return "", result.err
	}

	if result.success {
		return result.successMsg, nil
	}

	return "", nil
}
