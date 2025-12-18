package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CancellationError struct{}

func (e *CancellationError) Error() string {
	return "operation cancelled"
}

type createStep int

const (
	stepMode createStep = iota
	stepGPU
	stepCompute
	stepTemplate
	stepDiskSize
	stepConfirmation
	stepComplete
)

// CreateConfig holds the configuration for creating an instance
type CreateConfig struct {
	Mode       string
	GPUType    string
	NumGPUs    int
	VCPUs      int
	Template   string
	DiskSizeGB int
	Confirmed  bool
}

type createModel struct {
	step            createStep
	cursor          int
	config          CreateConfig
	templates       []api.Template
	snapshots       []api.Snapshot
	templatesLoaded bool
	snapshotsLoaded bool
	diskInput       textinput.Model
	err             error
	quitting        bool
	client          *api.Client
	spinner         spinner.Model

	styles createStyles
}

type createStyles struct {
	title    lipgloss.Style
	selected lipgloss.Style
	cursor   lipgloss.Style
	panel    lipgloss.Style
	label    lipgloss.Style
	help     lipgloss.Style
}

func newCreateStyles() createStyles {
	panelBase := PrimaryStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.PrimaryColor)).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	return createStyles{
		title:    PrimaryTitleStyle().MarginBottom(1),
		selected: PrimarySelectedStyle(),
		cursor:   PrimaryCursorStyle(),
		panel:    panelBase,
		label:    LabelStyle(),
		help:     HelpStyle(),
	}
}

func NewCreateModel(client *api.Client) createModel {
	styles := newCreateStyles()

	ti := textinput.New()
	ti.Placeholder = "100"
	ti.CharLimit = 4
	ti.Width = 20
	ti.Prompt = "▶ "
	ti.PromptStyle = styles.cursor
	ti.TextStyle = styles.cursor
	ti.PlaceholderStyle = styles.cursor
	ti.Cursor.Style = styles.cursor

	s := NewPrimarySpinner()

	return createModel{
		step:      stepMode,
		client:    client,
		diskInput: ti,
		spinner:   s,
		styles:    styles,
		config: CreateConfig{
			DiskSizeGB: 100,
		},
	}
}

type createTemplatesMsg struct {
	templates []api.Template
	err       error
}

type createSnapshotsMsg struct {
	snapshots []api.Snapshot
	err       error
}

func sortTemplates(templates []api.Template) []api.Template {
	sorted := make([]api.Template, 0, len(templates))

	for _, t := range templates {
		if t.Key == "base" || strings.EqualFold(t.Key, "base") {
			sorted = append(sorted, t)
			break
		}
	}

	for _, t := range templates {
		if t.Key != "base" && !strings.EqualFold(t.Key, "base") {
			sorted = append(sorted, t)
		}
	}

	return sorted
}

func fetchCreateTemplatesCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		templates, err := client.ListTemplates()
		if err == nil {
			templates = sortTemplates(templates)
		}
		return createTemplatesMsg{templates: templates, err: err}
	}
}

func fetchCreateSnapshotsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		snapshots, err := client.ListSnapshots()
		// Filter for READY snapshots only
		if err == nil {
			readySnapshots := make([]api.Snapshot, 0)
			for _, s := range snapshots {
				if s.Status == "READY" {
					readySnapshots = append(readySnapshots, s)
				}
			}
			snapshots = readySnapshots
		}
		return createSnapshotsMsg{snapshots: snapshots, err: err}
	}
}

func (m createModel) Init() tea.Cmd {
	return tea.Batch(fetchCreateTemplatesCmd(m.client), fetchCreateSnapshotsCmd(m.client), m.spinner.Tick)
}

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case createTemplatesMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.templates = msg.templates
		m.templatesLoaded = true
		if len(m.templates) == 0 {
			m.err = fmt.Errorf("no templates available")
			return m, tea.Quit
		}
		return m, m.spinner.Tick

	case createSnapshotsMsg:
		// Snapshots are optional, so ignore errors
		if msg.err == nil {
			m.snapshots = msg.snapshots
		}
		m.snapshotsLoaded = true
		return m, m.spinner.Tick

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		// Keep spinning if templates or snapshots haven't loaded yet
		if !m.templatesLoaded || !m.snapshotsLoaded {
			return m, tea.Batch(cmd, m.spinner.Tick)
		}
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.step > stepMode {
				m.step--
				m.cursor = 0
				if m.step == stepDiskSize {
					m.diskInput.Blur()
				}
			} else if m.step == stepMode {
				m.quitting = true
				return m, tea.Quit
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.step != stepDiskSize && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			maxCursor := m.getMaxCursor()
			if m.step != stepDiskSize && m.cursor < maxCursor {
				m.cursor++
			}
		}
	}

	if m.step == stepDiskSize {
		var cmd tea.Cmd
		m.diskInput, cmd = m.diskInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m createModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepMode:
		modes := []string{"prototyping", "production"}
		m.config.Mode = modes[m.cursor]
		m.step = stepGPU
		m.cursor = 0

	case stepGPU:
		gpus := m.getGPUOptions()
		m.config.GPUType = gpus[m.cursor]
		m.step = stepCompute
		m.cursor = 0

	case stepCompute:
		if m.config.Mode == "prototyping" {
			vcpus := []int{4, 8, 16, 32}
			m.config.VCPUs = vcpus[m.cursor]
			m.config.NumGPUs = 1
		} else {
			numGPUs := []int{1, 2, 4}
			m.config.NumGPUs = numGPUs[m.cursor]
			m.config.VCPUs = 18 * m.config.NumGPUs
		}
		m.step = stepTemplate
		m.cursor = 0

	case stepTemplate:
		totalOptions := len(m.templates) + len(m.snapshots)
		if m.cursor < totalOptions {
			// Check if cursor is on a template or snapshot
			if m.cursor < len(m.templates) {
				// Selected a template
				m.config.Template = m.templates[m.cursor].Key
				m.diskInput.SetValue("100")
			} else {
				// Selected a snapshot
				snapshotIndex := m.cursor - len(m.templates)
				snapshot := m.snapshots[snapshotIndex]
				m.config.Template = snapshot.Name
				// Set minimum disk size from snapshot
				m.diskInput.SetValue(fmt.Sprintf("%d", snapshot.MinimumDiskSizeGB))
			}
			m.step = stepDiskSize
			m.diskInput.Focus()
		}

	case stepDiskSize:
		diskSize, err := strconv.Atoi(m.diskInput.Value())
		if err != nil || diskSize < 100 || diskSize > 1000 {
			m.err = fmt.Errorf("disk size must be between 100 and 1000 GB")
			return m, nil
		}
		m.config.DiskSizeGB = diskSize
		m.err = nil
		m.step = stepConfirmation
		m.cursor = 0
		m.diskInput.Blur()

	case stepConfirmation:
		if m.cursor == 0 {
			// Create instance
			m.config.Confirmed = true
			m.step = stepComplete
			return m, tea.Quit
		}
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m createModel) getGPUOptions() []string {
	if m.config.Mode == "prototyping" {
		return []string{"t4", "a100xl"}
	}
	return []string{"a100xl", "h100"}
}

func (m createModel) getMaxCursor() int {
	switch m.step {
	case stepMode:
		return 1
	case stepGPU:
		return len(m.getGPUOptions()) - 1
	case stepCompute:
		if m.config.Mode == "prototyping" {
			return 3
		}
		return 2
	case stepTemplate:
		return len(m.templates) + len(m.snapshots) - 1
	case stepConfirmation:
		return 1
	}
	return 0
}

func (m createModel) View() string {
	if m.err != nil {
		return ""
	}

	if m.quitting {
		return ""
	}

	if m.step == stepComplete {
		return ""
	}

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(m.styles.title.Render("⚡ Create Thunder Compute Instance"))
	s.WriteString("\n")

	progressSteps := []string{"Mode", "GPU", "Compute", "Template", "Disk", "Confirm"}
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
	case stepMode:
		s.WriteString("Select instance mode:\n\n")
		modes := []string{"Prototyping (lowest cost, dev/test)", "Production (highest stability, long-running)"}
		for i, mode := range modes {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}
			display := mode
			if m.cursor == i {
				display = m.styles.selected.Render(mode)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, display))
		}

	case stepGPU:
		s.WriteString("Select GPU type:\n\n")
		gpus := m.getGPUOptions()
		for i, gpu := range gpus {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}
			displayName := strings.ToUpper(gpu)

			switch gpu {
			case "a100xl":
				switch m.config.Mode {
				case "production":
					displayName = "A100 80GB"
				case "prototyping":
					displayName = "A100 80GB (more powerful)"
				}
			case "h100":
				if m.config.Mode == "production" {
					displayName = "H100"
				}
			case "t4":
				if m.config.Mode == "prototyping" {
					displayName += " (more affordable)"
				}
			}
			if m.cursor == i {
				displayName = m.styles.selected.Render(displayName)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, displayName))
		}

	case stepCompute:
		if m.config.Mode == "prototyping" {
			s.WriteString("Select vCPU count (8GB RAM per vCPU):\n\n")
			vcpus := []int{4, 8, 16, 32}
			for i, vcpu := range vcpus {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.cursor.Render("▶ ")
				}
				ram := vcpu * 8
				line := fmt.Sprintf("%s%d vCPUs (%d GB RAM)", cursor, vcpu, ram)
				if m.cursor == i {
					line = fmt.Sprintf("%s%s", cursor, m.styles.selected.Render(fmt.Sprintf("%d vCPUs (%d GB RAM)", vcpu, ram)))
				}
				s.WriteString(line + "\n")
			}
		} else {
			s.WriteString("Select number of GPUs (18 vCPUs per GPU, 144GB RAM per GPU):\n\n")
			numGPUs := []int{1, 2, 4}
			for i, num := range numGPUs {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.cursor.Render("▶ ")
				}
				vcpus := num * 18
				text := fmt.Sprintf("%d GPU(s) → %d vCPUs", num, vcpus)
				if m.cursor == i {
					text = m.styles.selected.Render(text)
				}
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, text))
			}
		}

	case stepTemplate:
		s.WriteString("Select OS template or custom snapshot:\n\n")
		if !m.templatesLoaded || !m.snapshotsLoaded {
			s.WriteString(fmt.Sprintf("%s Loading options...\n", m.spinner.View()))
		} else {
			// Display templates first
			s.WriteString(m.styles.label.Render("Templates:") + "\n")
			for i, template := range m.templates {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.cursor.Render("▶ ")
				}
				name := template.DisplayName
				if template.ExtendedDescription != "" {
					name += fmt.Sprintf(" - %s", template.ExtendedDescription)
				}
				if m.cursor == i {
					name = m.styles.selected.Render(name)
				}
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
			}

			// Display snapshots if any
			if len(m.snapshots) > 0 {
				s.WriteString("\n")
				s.WriteString(m.styles.label.Render("Custom Snapshots:") + "\n")
				for i, snapshot := range m.snapshots {
					cursorIndex := len(m.templates) + i
					cursor := "  "
					if m.cursor == cursorIndex {
						cursor = m.styles.cursor.Render("▶ ")
					}
					name := fmt.Sprintf("%s (%d GB)", snapshot.Name, snapshot.MinimumDiskSizeGB)
					if m.cursor == cursorIndex {
						name = m.styles.selected.Render(name)
					}
					s.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
				}
			}
		}

	case stepDiskSize:
		s.WriteString("Enter disk size (GB):\n\n")
		s.WriteString("Range: 100-1000 GB\n\n")
		s.WriteString(m.diskInput.View())
		s.WriteString("\n\n")
		if m.err != nil {
			s.WriteString(errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v", m.err)))
			s.WriteString("\n")
		}
		s.WriteString(m.styles.help.Render("Press Enter to continue\n"))

	case stepConfirmation:
		s.WriteString("Review your configuration:\n")

		var panel strings.Builder
		panel.WriteString(m.styles.label.Render("Mode:       ") + utils.Capitalize(m.config.Mode) + "\n")
		panel.WriteString(m.styles.label.Render("GPU Type:   ") + strings.ToUpper(m.config.GPUType) + "\n")
		panel.WriteString(m.styles.label.Render("GPUs:       ") + strconv.Itoa(m.config.NumGPUs) + "\n")
		panel.WriteString(m.styles.label.Render("vCPUs:      ") + strconv.Itoa(m.config.VCPUs) + "\n")
		panel.WriteString(m.styles.label.Render("RAM:        ") + strconv.Itoa(m.config.VCPUs*8) + " GB\n")
		panel.WriteString(m.styles.label.Render("Template:   ") + utils.Capitalize(m.config.Template) + "\n")
		panel.WriteString(m.styles.label.Render("Disk Size:  ") + strconv.Itoa(m.config.DiskSizeGB) + " GB")

		s.WriteString(m.styles.panel.Render(panel.String()))
		s.WriteString("\n")

		if m.config.Mode == "prototyping" {
			warning := "⚠ Prototyping mode: for dev/testing; not for production inference or long-running tasks.\n"
			s.WriteString(warningStyleTUI.Render(warning))
			s.WriteString("\n")
		}

		s.WriteString("Confirm creation?\n\n")
		options := []string{"✓ Create Instance", "✗ Cancel"}

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

	if m.step != stepConfirmation {
		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Cancel\n"))
	} else {
		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Confirm  Q: Cancel\n"))
	}

	return s.String()
}

func RunCreateInteractive(client *api.Client) (*CreateConfig, error) {
	InitCommonStyles(os.Stdout)
	m := NewCreateModel(client)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(createModel)

	if result.err != nil {
		return nil, result.err
	}

	if result.quitting || !result.config.Confirmed {
		return nil, &CancellationError{}
	}

	return &result.config, nil
}
