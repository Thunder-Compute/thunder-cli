package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
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
	ti.Placeholder = "e.g., 8080, 3000, 8443"
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
			portStrs := make([]string, len(m.currentPorts))
			for i, p := range m.currentPorts {
				portStrs[i] = fmt.Sprintf("%d", p)
			}
			m.portInput.SetValue(strings.Join(portStrs, ", "))
		}
		m.step = portsForwardStepEditPorts
		m.cursor = 0
		m.portInput.Focus()
		return m, nil

	case portsForwardStepEditPorts:
		// Parse the new ports
		newPorts, err := parsePortsInput(m.portInput.Value())
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

func parsePortsInput(input string) ([]int, error) {
	if strings.TrimSpace(input) == "" {
		return []int{}, nil
	}

	parts := strings.Split(input, ",")
	ports := make([]int, 0, len(parts))
	seen := make(map[int]bool)

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		port, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", p)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("port %d out of range (1-65535)", port)
		}
		if port == 22 {
			return nil, fmt.Errorf("port 22 is reserved for SSH")
		}
		if !seen[port] {
			ports = append(ports, port)
			seen[port] = true
		}
	}

	return ports, nil
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
			portStrs := make([]string, len(instance.HTTPPorts))
			for j, p := range instance.HTTPPorts {
				portStrs[j] = fmt.Sprintf("%d", p)
			}
			portsStr = strings.Join(portStrs, ", ")
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

	s.WriteString("Enter the ports to forward (comma-separated):\n")
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
		portStrs := make([]string, len(m.removePorts))
		for i, p := range m.removePorts {
			portStrs[i] = fmt.Sprintf("%d", p)
		}
		panel.WriteString("\n\n")
		panel.WriteString(m.styles.label.Render("Remove:") + "        " + strings.Join(portStrs, ", "))
	}

	if len(m.addPorts) > 0 {
		portStrs := make([]string, len(m.addPorts))
		for i, p := range m.addPorts {
			portStrs[i] = fmt.Sprintf("%d", p)
		}
		if len(m.removePorts) == 0 {
			panel.WriteString("\n\n")
		} else {
			panel.WriteString("\n")
		}
		panel.WriteString(m.styles.label.Render("Add:") + "           " + strings.Join(portStrs, ", "))
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
		portStrs := make([]string, len(m.resp.HTTPPorts))
		for i, p := range m.resp.HTTPPorts {
			portStrs[i] = fmt.Sprintf("%d", p)
		}
		lines = append(lines, labelStyle.Render("Forwarded Ports:")+" "+valueStyle.Render(strings.Join(portStrs, ", ")))
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
