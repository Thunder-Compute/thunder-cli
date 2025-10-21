package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joshuawatkins04/thunder-cli-draft/api"
)

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
	step      createStep
	cursor    int
	config    CreateConfig
	templates []api.Template
	diskInput textinput.Model
	err       error
	quitting  bool
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0391ff")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0391ff")).
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0391ff"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#0391ff")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FFA500")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)
)

func NewCreateModel(templates []api.Template) createModel {
	ti := textinput.New()
	ti.Placeholder = "100"
	ti.CharLimit = 4
	ti.Width = 20

	return createModel{
		step:      stepMode,
		templates: templates,
		diskInput: ti,
		config: CreateConfig{
			DiskSizeGB: 100,
		},
	}
}

func (m createModel) Init() tea.Cmd {
	return nil
}

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.step != stepConfirmation {
				m.quitting = true
				return m, tea.Quit
			}

		case "esc":
			if m.step > stepMode && m.step < stepConfirmation {
				m.step--
				m.cursor = 0
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
		if m.cursor < len(m.templates) {
			m.config.Template = m.templates[m.cursor].Key
			m.step = stepDiskSize
			m.diskInput.Focus()
			m.diskInput.SetValue("100")
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
			m.config.Confirmed = true
			m.step = stepComplete
			return m, tea.Quit
		} else {
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m createModel) getGPUOptions() []string {
	if m.config.Mode == "prototyping" {
		return []string{"t4", "a100"}
	}
	return []string{"a100", "h100"}
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
		return len(m.templates) - 1
	case stepConfirmation:
		return 1
	}
	return 0
}

func (m createModel) View() string {
	if m.quitting {
		return "Operation cancelled.\n"
	}

	if m.step == stepComplete {
		return ""
	}

	var s strings.Builder

	s.WriteString(titleStyle.Render("⚡ Create Thunder Compute Instance"))
	s.WriteString("\n\n")

	progressSteps := []string{"Mode", "GPU", "Compute", "Template", "Disk", "Confirm"}
	progress := ""
	for i, stepName := range progressSteps {
		if i == int(m.step) {
			progress += selectedStyle.Render(fmt.Sprintf("[%s]", stepName))
		} else if i < int(m.step) {
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
				cursor = cursorStyle.Render("▶ ")
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, mode))
		}

	case stepGPU:
		s.WriteString("Select GPU type:\n\n")
		gpus := m.getGPUOptions()
		for i, gpu := range gpus {
			cursor := "  "
			if m.cursor == i {
				cursor = cursorStyle.Render("▶ ")
			}
			displayName := strings.ToUpper(gpu)
			if m.config.Mode == "prototyping" {
				if gpu == "t4" {
					displayName += " (more affordable)"
				} else {
					displayName += " (more powerful)"
				}
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
					cursor = cursorStyle.Render("▶ ")
				}
				ram := vcpu * 8
				s.WriteString(fmt.Sprintf("%s%d vCPUs (%d GB RAM)\n", cursor, vcpu, ram))
			}
		} else {
			s.WriteString("Select number of GPUs (18 vCPUs per GPU, 144GB RAM per GPU):\n\n")
			numGPUs := []int{1, 2, 4}
			for i, num := range numGPUs {
				cursor := "  "
				if m.cursor == i {
					cursor = cursorStyle.Render("▶ ")
				}
				vcpus := num * 18
				s.WriteString(fmt.Sprintf("%s%d GPU(s) → %d vCPUs\n", cursor, num, vcpus))
			}
		}

	case stepTemplate:
		s.WriteString("Select OS template:\n\n")
		for i, template := range m.templates {
			cursor := "  "
			if m.cursor == i {
				cursor = cursorStyle.Render("▶ ")
			}
			name := template.DisplayName
			if template.ExtendedDescription != "" {
				name += fmt.Sprintf(" - %s", template.ExtendedDescription)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
		}

	case stepDiskSize:
		s.WriteString("Enter disk size (GB):\n\n")
		s.WriteString("Range: 100-1000 GB\n\n")
		s.WriteString(m.diskInput.View())
		s.WriteString("\n\n")
		if m.err != nil {
			s.WriteString(errorStyle.Render(m.err.Error()))
			s.WriteString("\n")
		}
		s.WriteString("Press Enter to continue\n")

	case stepConfirmation:
		s.WriteString("Review your configuration:\n\n")

		panel := fmt.Sprintf(
			"Mode:       %s\n"+
				"GPU Type:   %s\n"+
				"GPUs:       %d\n"+
				"vCPUs:      %d\n"+
				"RAM:        %d GB\n"+
				"Template:   %s\n"+
				"Disk Size:  %d GB",
			strings.Title(m.config.Mode),
			strings.ToUpper(m.config.GPUType),
			m.config.NumGPUs,
			m.config.VCPUs,
			m.config.VCPUs*8,
			m.getTemplateName(),
			m.config.DiskSizeGB,
		)
		s.WriteString(panelStyle.Render(panel))
		s.WriteString("\n\n")

		if m.config.Mode == "prototyping" {
			warning := "PROTOTYPING MODE DISCLAIMER\n\n" +
				"Prototyping instances are designed for development and testing.\n" +
				"They may experience occasional interruptions and are not recommended\n" +
				"for production workloads or long-running tasks."
			s.WriteString(warningStyle.Render(warning))
			s.WriteString("\n\n")
		}

		s.WriteString("Confirm creation?\n\n")
		options := []string{"✓ Create Instance", "✗ Cancel"}
		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = cursorStyle.Render("▶ ")
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
		}
	}

	if m.step != stepConfirmation {
		s.WriteString("\n")
		s.WriteString("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Quit\n")
	} else {
		s.WriteString("\n")
		s.WriteString("↑/↓: Navigate  Enter: Confirm\n")
	}

	return s.String()
}

func (m createModel) getTemplateName() string {
	for _, t := range m.templates {
		if t.Key == m.config.Template {
			return t.DisplayName
		}
	}
	return m.config.Template
}

func RunCreateInteractive(templates []api.Template) (*CreateConfig, error) {
	m := NewCreateModel(templates)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(createModel)
	if result.quitting || !result.config.Confirmed {
		return nil, fmt.Errorf("operation cancelled")
	}

	return &result.config, nil
}
