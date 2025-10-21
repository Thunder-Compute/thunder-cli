package tui

import (
	"fmt"
	"strings"

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

func NewDeleteModel(instances []api.Instance) deleteModel {
	return deleteModel{
		step:      deleteStepSelect,
		instances: instances,
	}
}

func (m deleteModel) Init() tea.Cmd {
	return nil
}

func (m deleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
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
	if m.quitting {
		return "Operation cancelled.\n"
	}

	if m.step == deleteStepComplete {
		return ""
	}

	var s strings.Builder

	s.WriteString(deleteTitleStyle.Render("🗑️  Delete Thunder Compute Instance"))
	s.WriteString("\n\n")

	switch m.step {
	case deleteStepSelect:
		if len(m.instances) == 0 {
			s.WriteString("No instances found.\n")
			s.WriteString("\nPress q to quit.\n")
			return s.String()
		}

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

func RunDeleteInteractive(instances []api.Instance) (*api.Instance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available to delete")
	}

	m := NewDeleteModel(instances)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(deleteModel)
	if result.quitting || !result.confirmed || result.selected == nil {
		return nil, fmt.Errorf("operation cancelled")
	}

	return result.selected, nil
}
