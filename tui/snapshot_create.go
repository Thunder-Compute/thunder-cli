package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type snapshotCreateStep int

const (
	snapshotCreateStepSelectInstance snapshotCreateStep = iota
	snapshotCreateStepEnterName
	snapshotCreateStepConfirm
	snapshotCreateStepComplete
)

// SnapshotCreateConfig holds the configuration for creating a snapshot
type SnapshotCreateConfig struct {
	InstanceID string
	Name       string
	Confirmed  bool
}

type snapshotCreateModel struct {
	step             snapshotCreateStep
	cursor           int
	config           SnapshotCreateConfig
	instances        []api.Instance
	runningInstances []api.Instance
	instancesLoaded  bool
	nameInput        textinput.Model
	err              error
	validationErr    error
	quitting         bool
	client           *api.Client
	spinner          spinner.Model

	styles snapshotCreateStyles
}

type snapshotCreateStyles struct {
	title      lipgloss.Style
	selected   lipgloss.Style
	cursor     lipgloss.Style
	panel      lipgloss.Style
	label      lipgloss.Style
	help       lipgloss.Style
	warningBox lipgloss.Style
}

func newSnapshotCreateStyles() snapshotCreateStyles {
	panelBase := PrimaryStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.PrimaryColor)).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	return snapshotCreateStyles{
		title:      PrimaryTitleStyle().MarginBottom(1),
		selected:   PrimarySelectedStyle(),
		cursor:     PrimaryCursorStyle(),
		panel:      panelBase,
		label:      LabelStyle(),
		help:       HelpStyle(),
		warningBox: WarningBoxStyle().MarginTop(1).MarginBottom(1),
	}
}

func NewSnapshotCreateModel(client *api.Client) snapshotCreateModel {
	styles := newSnapshotCreateStyles()

	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 50
	ti.Width = 40
	ti.Prompt = "▶ "
	ti.PromptStyle = styles.cursor
	ti.TextStyle = styles.cursor
	ti.PlaceholderStyle = styles.cursor
	ti.Cursor.Style = styles.cursor

	s := NewPrimarySpinner()

	return snapshotCreateModel{
		step:      snapshotCreateStepSelectInstance,
		client:    client,
		nameInput: ti,
		spinner:   s,
		styles:    styles,
	}
}

type snapshotCreateInstancesMsg struct {
	instances []api.Instance
	err       error
}

func fetchSnapshotCreateInstancesCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		instances, err := client.ListInstances()
		return snapshotCreateInstancesMsg{instances: instances, err: err}
	}
}

func (m snapshotCreateModel) Init() tea.Cmd {
	return tea.Batch(fetchSnapshotCreateInstancesCmd(m.client), m.spinner.Tick)
}

func (m snapshotCreateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case snapshotCreateInstancesMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.instances = msg.instances
		// Filter for RUNNING instances only
		m.runningInstances = make([]api.Instance, 0)
		for _, inst := range m.instances {
			if inst.Status == "RUNNING" {
				m.runningInstances = append(m.runningInstances, inst)
			}
		}
		// Sort by ID (same as tnr status)
		if len(m.runningInstances) > 1 {
			sort.Slice(m.runningInstances, func(i, j int) bool {
				return m.runningInstances[i].ID < m.runningInstances[j].ID
			})
		}
		m.instancesLoaded = true
		if len(m.runningInstances) == 0 {
			m.err = fmt.Errorf("no running instances found. Snapshots can only be created from instances in RUNNING state")
			return m, tea.Quit
		}
		return m, m.spinner.Tick

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if !m.instancesLoaded {
			return m, tea.Batch(cmd, m.spinner.Tick)
		}
		return m, cmd

	case tea.KeyMsg:
		// If we're on the name input step and the input is focused, don't handle q/Q as quit
		if m.step == snapshotCreateStepEnterName && m.nameInput.Focused() {
			// Let the text input handle all keys except ctrl+c, esc, and enter
			if msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			if msg.String() == "esc" {
				// Blur and go back
				m.nameInput.Blur()
				m.step--
				m.cursor = 0
				m.validationErr = nil
				return m, nil
			}
			if msg.String() == "enter" {
				// Handle enter to submit the name
				return m.handleEnter()
			}
			// Pass all other keys to the text input
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			if m.validationErr != nil && m.nameInput.Value() != "" {
				m.validationErr = nil
			}
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.step > snapshotCreateStepSelectInstance {
				// If currently on name input, blur it
				if m.step == snapshotCreateStepEnterName {
					m.nameInput.Blur()
				}
				m.step--
				m.cursor = 0
				m.validationErr = nil // Clear any validation errors when going back
			} else {
				m.quitting = true
				return m, tea.Quit
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.step != snapshotCreateStepEnterName && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			maxCursor := m.getMaxCursor()
			if m.step != snapshotCreateStepEnterName && m.cursor < maxCursor {
				m.cursor++
			}
		}
	}

	return m, nil
}

func (m snapshotCreateModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case snapshotCreateStepSelectInstance:
		if m.cursor < len(m.runningInstances) {
			m.config.InstanceID = m.runningInstances[m.cursor].UUID
			m.step = snapshotCreateStepEnterName
			m.nameInput.Focus()
		}

	case snapshotCreateStepEnterName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.validationErr = fmt.Errorf("snapshot name cannot be empty")
			// Don't advance to next step, stay on name input
			return m, nil
		}
		m.config.Name = name
		m.validationErr = nil
		m.step = snapshotCreateStepConfirm
		m.cursor = 0
		m.nameInput.Blur()

	case snapshotCreateStepConfirm:
		if m.cursor == 0 {
			// Create snapshot
			m.config.Confirmed = true
			m.step = snapshotCreateStepComplete
			return m, tea.Quit
		}
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m snapshotCreateModel) getMaxCursor() int {
	switch m.step {
	case snapshotCreateStepSelectInstance:
		return len(m.runningInstances) - 1
	case snapshotCreateStepConfirm:
		return 1
	}
	return 0
}

func (m snapshotCreateModel) View() string {
	if m.err != nil {
		return ""
	}

	if m.quitting {
		return ""
	}

	if m.step == snapshotCreateStepComplete {
		return ""
	}

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(m.styles.title.Render("⚡ Create Snapshot"))
	s.WriteString("\n")

	// Beta notice
	betaNotice := "ℹ Snapshots are currently in beta. Please share feedback with us on Discord (https://discord.gg/nwuETS9jJK) or by emailing support@thundercompute.com"
	s.WriteString(WarningStyle().Width(80).Render(betaNotice))
	s.WriteString("\n\n")

	progressSteps := []string{"Instance", "Name", "Confirm"}
	progress := ""
	for i, stepName := range progressSteps {
		adjustedStep := int(m.step)
		if i == adjustedStep {
			progress += m.styles.selected.Render(fmt.Sprintf("[%s]", stepName))
		} else if i < adjustedStep {
			progress += fmt.Sprintf("[✓ %s]", stepName)
		} else {
			progress += fmt.Sprintf("[%s]", stepName)
		}
		if i < len(progressSteps)-1 {
			progress += " → "
		}
	}
	s.WriteString(progress)
	s.WriteString("\n\n")

	switch m.step {
	case snapshotCreateStepSelectInstance:
		s.WriteString("Select a running instance to snapshot:\n\n")
		if !m.instancesLoaded {
			s.WriteString(fmt.Sprintf("%s Loading instances...\n", m.spinner.View()))
		} else {
			for i, instance := range m.runningInstances {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.cursor.Render("▶ ")
				}

				display := fmt.Sprintf("(%s) %s - %s - %sx%s",
					instance.ID,
					instance.Name,
					instance.IP,
					instance.NumGPUs,
					instance.GPUType,
				)
				if m.cursor == i {
					display = m.styles.selected.Render(display)
				}
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, display))
			}
		}

	case snapshotCreateStepEnterName:
		s.WriteString("Enter a name for the snapshot:\n\n")
		s.WriteString(m.nameInput.View())
		s.WriteString("\n\n")
		if m.validationErr != nil {
			s.WriteString(errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v", m.validationErr)))
			s.WriteString("\n")
		}
		s.WriteString(m.styles.help.Render("Press Enter to continue\n"))

	case snapshotCreateStepConfirm:
		s.WriteString("Review your snapshot configuration:\n")

		var panel strings.Builder
		// Find the instance details
		var selectedInstance *api.Instance
		for i := range m.runningInstances {
			if m.runningInstances[i].UUID == m.config.InstanceID {
				selectedInstance = &m.runningInstances[i]
				break
			}
		}

		if selectedInstance != nil {
			panel.WriteString(m.styles.label.Render("Instance ID:   ") + selectedInstance.ID + "\n")
			panel.WriteString(m.styles.label.Render("Instance Name: ") + selectedInstance.Name + "\n")
			panel.WriteString(m.styles.label.Render("Instance IP:   ") + selectedInstance.IP + "\n")
		}
		panel.WriteString(m.styles.label.Render("Snapshot Name: ") + m.config.Name)

		s.WriteString(m.styles.panel.Render(panel.String()))
		s.WriteString("\n")

		warning := "⚠ This will terminate running tasks and pause the instance."
		s.WriteString(m.styles.warningBox.Render(warning))
		s.WriteString("\n\n")

		s.WriteString("Confirm snapshot creation?\n\n")
		options := []string{"✓ Create Snapshot", "✗ Cancel"}

		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}
			text := option
			if m.cursor == i {
				text = m.styles.selected.Render(option)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, text))
		}
	}

	if m.step != snapshotCreateStepConfirm {
		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Cancel\n"))
	} else {
		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Confirm  Q: Cancel\n"))
	}

	return s.String()
}

func RunSnapshotCreateInteractive(client *api.Client) (*SnapshotCreateConfig, error) {
	InitCommonStyles(os.Stdout)
	m := NewSnapshotCreateModel(client)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(snapshotCreateModel)

	if result.err != nil {
		return nil, result.err
	}

	if result.quitting || !result.config.Confirmed {
		return nil, &CancellationError{}
	}

	return &result.config, nil
}
