package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joshuawatkins04/thunder-cli-draft/api"
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
				MarginBottom(1)

	deleteSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0391ff")).
				Bold(true)

	deleteCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0391ff"))

	deleteWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF0000")).
				Bold(true).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FF0000")).
				Padding(1, 2).
				MarginTop(1).
				MarginBottom(1)

	deleteInstanceStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#0391ff")).
				Padding(1, 2).
				MarginTop(1).
				MarginBottom(1)
)

func NewDeleteModel(client *api.Client) deleteModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0391ff"))

	return deleteModel{
		step:    deleteStepSelect,
		client:  client,
		loading: true,
		spinner: s,
	}
}

type deleteInstancesMsg struct {
	instances []api.Instance
	err       error
}

func fetchDeleteInstancesCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		instances, err := client.ListInstances()
		return deleteInstancesMsg{instances: instances, err: err}
	}
}

func (m deleteModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchDeleteInstancesCmd(m.client))
}

func (m deleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case deleteInstancesMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.instances = msg.instances

		if len(m.instances) == 0 {
			m.quitting = true
			return m, tea.Quit
		}

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
			if m.step != deleteStepConfirm {
				m.quitting = true
				return m, tea.Quit
			}

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
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	if m.loading {
		return "\n  " + m.spinner.View() + " Fetching instances...\n\n" + helpStyle.Render("Press q to cancel") + "\n"
	}

	if m.quitting {
		if len(m.instances) == 0 {
			return "No instances found.\n"
		}
		return ""
	}

	if m.step == deleteStepComplete {
		return ""
	}

	var s strings.Builder

	s.WriteString(deleteTitleStyle.Render("⚡ Delete Thunder Compute Instance ⚡"))
	s.WriteString("\n\n")

	switch m.step {
	case deleteStepSelect:
		s.WriteString("Select an instance to delete:\n\n")

		hasStartingInstances := false
		for i, instance := range m.instances {
			cursor := "  "
			if m.cursor == i {
				cursor = deleteCursorStyle.Render("▶ ")
			}

			statusColor := ""
			statusSuffix := ""
			switch instance.Status {
			case "RUNNING":
				statusColor = "\033[32m" // Green
			case "STARTING":
				statusColor = "\033[33m" // Yellow
				statusSuffix = ""
				hasStartingInstances = true
			case "DELETING":
				statusColor = "\033[31m" // Red
				statusSuffix = " (already deleting)"
			}
			resetColor := "\033[0m"

			info := fmt.Sprintf("%s (%s%s%s%s) - %s - %sx%s - %s",
				instance.UUID,
				statusColor,
				instance.Status,
				resetColor,
				statusSuffix,
				instance.IP,
				instance.NumGPUs,
				instance.GPUType,
				capitalize(instance.Mode),
			)

			s.WriteString(fmt.Sprintf("%s%s\n", cursor, info))
		}

		if hasStartingInstances {
			s.WriteString("\n")
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")).Render("Note: Instances in STARTING state may fail to delete. Wait until RUNNING."))
			s.WriteString("\n")
		}

		s.WriteString("\n↑/↓: Navigate  Enter: Select  Q: Cancel\n")

	case deleteStepConfirm:
		warning := "WARNING: This action is IRREVERSIBLE!\n\n" +
			"Deleting this instance will:\n" +
			"• Permanently destroy the instance and ALL data\n" +
			"• Remove all SSH configuration for this instance\n" +
			"• This action CANNOT be undone"
		s.WriteString(deleteWarningStyle.Render(warning))
		s.WriteString("\n\n")

		instanceInfo := fmt.Sprintf(
			"Instance ID:  %s\n"+
				"Status:       %s\n"+
				"IP Address:   %s\n"+
				"Mode:         %s\n"+
				"GPU:          %sx%s\n"+
				"Template:     %s",
			m.selected.UUID,
			m.selected.Status,
			m.selected.IP,
			capitalize(m.selected.Mode),
			m.selected.NumGPUs,
			m.selected.GPUType,
			m.selected.Template,
		)
		s.WriteString(deleteInstanceStyle.Render(instanceInfo))
		s.WriteString("\n\n")

		s.WriteString("Are you sure you want to delete this instance?\n\n")

		options := []string{"✓ Yes, Delete Instance", "✗ No, Cancel"}
		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = deleteCursorStyle.Render("▶ ")
			}
			if i == 0 {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(option)))
			} else {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
			}
		}

		s.WriteString("\n↑/↓: Navigate  Enter: Confirm  Esc: Back\n")
	}

	return s.String()
}

func RunDeleteInteractive(client *api.Client) (*api.Instance, error) {
	m := NewDeleteModel(client)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(deleteModel)

	if result.err != nil {
		return nil, result.err
	}

	if len(result.instances) == 0 {
		return nil, fmt.Errorf("no instances available to delete")
	}

	if result.quitting {
		return nil, &CancellationError{}
	}

	if !result.confirmed || result.selected == nil {
		return nil, &CancellationError{}
	}

	return result.selected, nil
}
