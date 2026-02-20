package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type portsForwardStep int

const (
	portsForwardStepSelectInstance portsForwardStep = iota
	portsForwardStepEditPorts
	portsForwardStepConfirmation
	portsForwardStepApplying
	portsForwardStepComplete
)

type portsForwardModel struct {
	step             portsForwardStep
	cursor           int
	instances        []api.Instance
	selectedInstance *api.Instance
	client           *api.Client
	portInput        textinput.Model
	currentPorts     []int
	addPorts         []int
	removePorts      []int
	err              error
	validationErr    error
	quitting         bool
	cancelled        bool
	spinner          spinner.Model
	resp             *api.InstanceModifyResponse

	styles portsForwardStyles
}

type portsForwardStyles struct {
	title    lipgloss.Style
	selected lipgloss.Style
	cursor   lipgloss.Style
	panel    lipgloss.Style
	label    lipgloss.Style
	help     lipgloss.Style
}

func newPortsForwardStyles() portsForwardStyles {
	panelBase := PrimaryStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.PrimaryColor)).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	return portsForwardStyles{
		title:    PrimaryTitleStyle().MarginBottom(1),
		selected: PrimarySelectedStyle(),
		cursor:   PrimaryCursorStyle(),
		panel:    panelBase,
		label:    LabelStyle(),
		help:     HelpStyle(),
	}
}

func NewPortsForwardModel(client *api.Client, instances []api.Instance) tea.Model {
	InitCommonStyles(os.Stdout)
	styles := newPortsForwardStyles()

	ti := textinput.New()
	ti.Placeholder = "e.g., 8080, 3000, 9000-9005"
	ti.CharLimit = 100
	ti.Width = 40
	ti.Prompt = "▶ "

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.Primary()

	return portsForwardModel{
		step:      portsForwardStepSelectInstance,
		cursor:    0,
		instances: instances,
		client:    client,
		portInput: ti,
		spinner:   s,
		styles:    styles,
	}
}

func (m portsForwardModel) Init() tea.Cmd {
	return nil
}

func (m portsForwardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		case "q":
			if m.step != portsForwardStepEditPorts && m.step != portsForwardStepApplying {
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			}

		case "esc":
			if m.step == portsForwardStepEditPorts {
				m.step = portsForwardStepSelectInstance
				m.cursor = 0
				m.validationErr = nil
				m.portInput.Blur()
				return m, nil
			} else if m.step == portsForwardStepConfirmation {
				m.step = portsForwardStepEditPorts
				m.cursor = 0
				m.portInput.Focus()
				return m, nil
			} else if m.step == portsForwardStepSelectInstance {
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			}

		case "up":
			if m.step == portsForwardStepSelectInstance {
				if m.cursor > 0 {
					m.cursor--
				}
			} else if m.step == portsForwardStepConfirmation {
				if m.cursor > 0 {
					m.cursor--
				}
			}

		case "down":
			if m.step == portsForwardStepSelectInstance {
				if m.cursor < len(m.instances)-1 {
					m.cursor++
				}
			} else if m.step == portsForwardStepConfirmation {
				if m.cursor < 1 {
					m.cursor++
				}
			}

		case "enter":
			return m.handleEnter()
		}

		// Handle text input for port editing step
		if m.step == portsForwardStepEditPorts {
			var cmd tea.Cmd
			m.portInput, cmd = m.portInput.Update(msg)
			return m, cmd
		}

	case portsForwardApiResultMsg:
		m.step = portsForwardStepComplete
		m.err = msg.err
		m.resp = msg.resp
		m.quitting = true
		return m, tea.Quit

	default:
		if m.step == portsForwardStepApplying {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m portsForwardModel) handleEnter() (tea.Model, tea.Cmd) {
	m.validationErr = nil

	switch m.step {
	case portsForwardStepSelectInstance:
		m.selectedInstance = &m.instances[m.cursor]
		m.currentPorts = m.selectedInstance.HTTPPorts
		// Pre-populate input with current ports
		if len(m.currentPorts) > 0 {
			m.portInput.SetValue(utils.FormatPorts(m.currentPorts))
		}
		m.step = portsForwardStepEditPorts
		m.cursor = 0
		m.portInput.Focus()
		return m, nil

	case portsForwardStepEditPorts:
		// Parse the new ports
		newPorts, err := utils.ParsePorts(m.portInput.Value())
		if err != nil {
			m.validationErr = err
			return m, nil
		}

		// Calculate add/remove ports
		m.addPorts, m.removePorts = calculatePortChanges(m.currentPorts, newPorts)

		if len(m.addPorts) == 0 && len(m.removePorts) == 0 {
			m.validationErr = fmt.Errorf("no changes to ports")
			return m, nil
		}

		m.step = portsForwardStepConfirmation
		m.cursor = 0
		m.portInput.Blur()
		return m, nil

	case portsForwardStepConfirmation:
		if m.cursor == 0 { // Apply Changes
			m.step = portsForwardStepApplying
			return m, tea.Batch(
				m.spinner.Tick,
				portsForwardApiCmd(m.client, m.selectedInstance.ID, m.addPorts, m.removePorts),
			)
		}
		// Cancel
		m.cancelled = true
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func calculatePortChanges(current, desired []int) (add, remove []int) {
	currentSet := make(map[int]bool)
	desiredSet := make(map[int]bool)

	for _, p := range current {
		currentSet[p] = true
	}
	for _, p := range desired {
		desiredSet[p] = true
	}

	// Ports to add: in desired but not in current
	for _, p := range desired {
		if !currentSet[p] {
			add = append(add, p)
		}
	}

	// Ports to remove: in current but not in desired
	for _, p := range current {
		if !desiredSet[p] {
			remove = append(remove, p)
		}
	}

	return add, remove
}

type portsForwardApiResultMsg struct {
	resp *api.InstanceModifyResponse
	err  error
}

func portsForwardApiCmd(client *api.Client, instanceID string, addPorts, removePorts []int) tea.Cmd {
	return func() tea.Msg {
		req := api.InstanceModifyRequest{
			AddPorts:    addPorts,
			RemovePorts: removePorts,
		}
		resp, err := client.ModifyInstance(instanceID, req)
		return portsForwardApiResultMsg{
			resp: resp,
			err:  err,
		}
	}
}

func (m portsForwardModel) View() string {
	if m.quitting && m.step != portsForwardStepComplete {
		return ""
	}

	var s strings.Builder

	switch m.step {
	case portsForwardStepSelectInstance:
		s.WriteString(m.renderSelectInstanceStep())
	case portsForwardStepEditPorts:
		s.WriteString(m.renderEditPortsStep())
	case portsForwardStepConfirmation:
		s.WriteString(m.renderConfirmationStep())
	case portsForwardStepApplying:
		s.WriteString(fmt.Sprintf("\n   %s Updating ports...\n\n", m.spinner.View()))
	case portsForwardStepComplete:
		s.WriteString(m.renderCompleteStep())
	}

	return s.String()
}

func (m portsForwardModel) renderSelectInstanceStep() string {
	var s strings.Builder

	s.WriteString(m.styles.title.Render("Forward HTTP Ports"))
	s.WriteString("\n")
	s.WriteString("Select an instance:\n\n")

	for i, instance := range m.instances {
		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.cursor.Render("▶ ")
		}

		// Format ports display
		portsStr := "(none)"
		if len(instance.HTTPPorts) > 0 {
			portsStr = utils.FormatPorts(instance.HTTPPorts)
		}

		idAndName := fmt.Sprintf("(%s) %s", instance.ID, instance.Name)
		if m.cursor == i {
			idAndName = m.styles.selected.Render(idAndName)
		}

		// Status style
		var statusStyle lipgloss.Style
		switch instance.Status {
		case "RUNNING":
			statusStyle = SuccessStyle()
		case "STARTING":
			statusStyle = WarningStyle()
		default:
			statusStyle = lipgloss.NewStyle()
		}

		statusText := statusStyle.Render(fmt.Sprintf("[%s]", instance.Status))
		rest := fmt.Sprintf(" %s  Ports: %s", statusText, portsStr)

		s.WriteString(fmt.Sprintf("%s%s%s\n", cursor, idAndName, rest))
	}

	s.WriteString("\n")
	s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  Q: Quit"))

	return s.String()
}

func (m portsForwardModel) renderEditPortsStep() string {
	var s strings.Builder

	s.WriteString(m.styles.title.Render("Forward HTTP Ports"))
	s.WriteString("\n")
	s.WriteString(m.styles.label.Render(fmt.Sprintf("Instance: (%s) %s", m.selectedInstance.ID, m.selectedInstance.Name)))
	s.WriteString("\n\n")

	s.WriteString("Enter the ports to forward (comma-separated or ranges like 8000-8005):\n")
	s.WriteString("Edit the list below to add or remove ports.\n\n")
	s.WriteString(m.portInput.View())
	s.WriteString("\n\n")

	if m.validationErr != nil {
		s.WriteString(errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v", m.validationErr)))
		s.WriteString("\n\n")
	}

	s.WriteString(m.styles.help.Render("Enter: Continue  ESC: Back  Ctrl+C: Quit"))

	return s.String()
}

func (m portsForwardModel) renderConfirmationStep() string {
	var s strings.Builder

	s.WriteString(m.styles.title.Render("Forward HTTP Ports"))

	valueStyle := lipgloss.NewStyle().Bold(true)

	var panel strings.Builder

	panel.WriteString(m.styles.label.Render("Instance ID:") + "   " + valueStyle.Render(m.selectedInstance.ID))
	panel.WriteString("\n")
	panel.WriteString(m.styles.label.Render("Instance UUID:") + " " + valueStyle.Render(m.selectedInstance.UUID))

	if len(m.removePorts) > 0 {
		panel.WriteString("\n\n")
		panel.WriteString(m.styles.label.Render("Remove:") + "        " + utils.FormatPorts(m.removePorts))
	}

	if len(m.addPorts) > 0 {
		if len(m.removePorts) == 0 {
			panel.WriteString("\n\n")
		} else {
			panel.WriteString("\n")
		}
		panel.WriteString(m.styles.label.Render("Add:") + "           " + utils.FormatPorts(m.addPorts))
	}

	s.WriteString(m.styles.panel.Render(panel.String()))
	s.WriteString("\n\nConfirm changes?\n\n")

	options := []string{"✓ Apply Changes", "✗ Cancel"}
	for i, option := range options {
		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.cursor.Render("▶ ")
			option = m.styles.selected.Render(option)
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
	}

	s.WriteString("\n")
	s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Confirm  ESC: Back"))

	return s.String()
}

func (m portsForwardModel) renderCompleteStep() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("\n✗ Failed to update ports: %v\n\n", m.err))
	}

	headerStyle := theme.Primary().Bold(true)
	labelStyle := theme.Neutral()
	valueStyle := lipgloss.NewStyle().Bold(true)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.PrimaryColor)).
		Padding(1, 2)

	var lines []string
	successTitleStyle := theme.Success()
	lines = append(lines, successTitleStyle.Render("✓ Ports updated successfully!"))
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("Instance ID:")+" "+valueStyle.Render(m.resp.Identifier))
	lines = append(lines, labelStyle.Render("Instance UUID:")+" "+valueStyle.Render(m.selectedInstance.UUID))

	if len(m.resp.HTTPPorts) > 0 {
		lines = append(lines, labelStyle.Render("Forwarded Ports:")+" "+valueStyle.Render(utils.FormatPorts(m.resp.HTTPPorts)))
	} else {
		lines = append(lines, labelStyle.Render("Forwarded Ports:")+" "+valueStyle.Render("(none)"))
	}

	lines = append(lines, "")
	lines = append(lines, headerStyle.Render("Access your services:"))
	if len(m.resp.HTTPPorts) > 0 {
		lines = append(lines, labelStyle.Render(fmt.Sprintf("  • https://%s-<port>.thundercompute.net", m.selectedInstance.UUID)))
	}
	lines = append(lines, labelStyle.Render("  • Run 'tnr ports list' to see all forwarded ports"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return "\n" + boxStyle.Render(content) + "\n\n"
}

// RunPortsForwardInteractive starts the interactive port forwarding flow
func RunPortsForwardInteractive(client *api.Client, instances []api.Instance) error {
	m := NewPortsForwardModel(client, instances)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running interactive port forward: %w", err)
	}

	finalPortsModel := finalModel.(portsForwardModel)

	if finalPortsModel.cancelled {
		return &CancellationError{}
	}

	if finalPortsModel.err != nil {
		return finalPortsModel.err
	}

	return nil
}
