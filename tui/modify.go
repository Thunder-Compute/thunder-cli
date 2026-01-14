package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
)

type modifyStep int

const (
	modifyStepMode modifyStep = iota
	modifyStepGPU
	modifyStepCompute
	modifyStepDiskSize
	modifyStepConfirmation
	modifyStepComplete
)

// ModifyConfig holds the configuration for modifying an instance
type ModifyConfig struct {
	Mode           string
	GPUType        string
	NumGPUs        int
	VCPUs          int
	DiskSizeGB     int
	Confirmed      bool
	ModeChanged    bool
	GPUChanged     bool
	ComputeChanged bool
	DiskChanged    bool
}

type modifyModel struct {
	step             modifyStep
	cursor           int
	config           ModifyConfig
	currentInstance  *api.Instance
	client           *api.Client
	diskInput        textinput.Model
	diskInputTouched bool
	err              error
	validationErr    error
	quitting         bool
	cancelled        bool

	styles modifyStyles
}

type modifyStyles struct {
	title    lipgloss.Style
	selected lipgloss.Style
	cursor   lipgloss.Style
	panel    lipgloss.Style
	label    lipgloss.Style
	help     lipgloss.Style
}

func newModifyStyles() modifyStyles {
	panelBase := PrimaryStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.PrimaryColor)).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	return modifyStyles{
		title:    PrimaryTitleStyle().MarginBottom(1),
		selected: PrimarySelectedStyle(),
		cursor:   PrimaryCursorStyle(),
		panel:    panelBase,
		label:    LabelStyle(),
		help:     HelpStyle(),
	}
}

func NewModifyModel(client *api.Client, instance *api.Instance) modifyModel {
	styles := newModifyStyles()

	ti := textinput.New()
	ti.Placeholder = fmt.Sprintf("%d", instance.Storage)
	ti.SetValue(fmt.Sprintf("%d", instance.Storage))
	ti.CharLimit = 4
	ti.Width = 20
	ti.Prompt = "▶ "

	m := modifyModel{
		step:             modifyStepMode,
		cursor:           0,
		config:           ModifyConfig{},
		currentInstance:  instance,
		client:           client,
		diskInput:        ti,
		diskInputTouched: false,
		styles:           styles,
	}

	// Set initial cursor to current mode position (case-insensitive)
	if strings.EqualFold(instance.Mode, "prototyping") {
		m.cursor = 0
	} else {
		m.cursor = 1
	}

	return m
}

func (m modifyModel) Init() tea.Cmd {
	return nil
}

func (m modifyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		case "q":
			if m.step == modifyStepConfirmation {
				// Q at confirmation should select cancel option
				break
			}
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.step > modifyStepMode {
				m.step--
				m.cursor = 0
				m.validationErr = nil
				if m.step == modifyStepDiskSize {
					m.diskInput.Focus()
					// Reset the touched flag when going back to disk size step
					m.diskInputTouched = false
				} else {
					m.diskInput.Blur()
				}
				return m, nil
			}
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		case "up":
			if m.step != modifyStepDiskSize {
				if m.cursor > 0 {
					m.cursor--
				}
			}

		case "down":
			if m.step != modifyStepDiskSize {
				maxCursor := m.getMaxCursor()
				if m.cursor < maxCursor {
					m.cursor++
				}
			}

		case "enter":
			return m.handleEnter()
		}

		// Handle text input for disk size step
		if m.step == modifyStepDiskSize {
			// Check if this is a character input (not a control key)
			if len(msg.String()) == 1 && msg.Type == tea.KeyRunes {
				// If this is the first character typed, clear the input first
				if !m.diskInputTouched {
					m.diskInput.SetValue("")
					m.diskInputTouched = true
				}
			}
			var cmd tea.Cmd
			m.diskInput, cmd = m.diskInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m modifyModel) handleEnter() (tea.Model, tea.Cmd) {
	m.validationErr = nil

	switch m.step {
	case modifyStepMode:
		modeOptions := []string{"prototyping", "production"}
		newMode := modeOptions[m.cursor]
		m.config.Mode = newMode
		// Case-insensitive comparison
		m.config.ModeChanged = !strings.EqualFold(newMode, m.currentInstance.Mode)
		m.step = modifyStepGPU
		// Set cursor to current GPU position for next step
		m.cursor = m.getCurrentGPUCursorPosition()
		return m, nil

	case modifyStepGPU:
		effectiveMode := m.currentInstance.Mode
		if m.config.ModeChanged {
			effectiveMode = m.config.Mode
		}

		var gpuValues []string
		if effectiveMode == "prototyping" {
			gpuValues = []string{"t4", "a100xl"}
		} else {
			gpuValues = []string{"a100xl", "h100"}
		}

		m.config.GPUType = gpuValues[m.cursor]
		// Case-insensitive comparison
		m.config.GPUChanged = !strings.EqualFold(m.config.GPUType, m.currentInstance.GPUType)
		m.step = modifyStepCompute
		// Set cursor to current compute position for next step
		m.cursor = m.getCurrentComputeCursorPosition()
		return m, nil

	case modifyStepCompute:
		effectiveMode := m.currentInstance.Mode
		if m.config.ModeChanged {
			effectiveMode = m.config.Mode
		}

		if effectiveMode == "prototyping" {
			vcpuOptions := []int{4, 8, 16, 32}
			m.config.VCPUs = vcpuOptions[m.cursor]
			currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
			m.config.ComputeChanged = (m.config.VCPUs != currentVCPUs)
		} else { // production
			gpuOptions := []int{1, 2, 4}
			m.config.NumGPUs = gpuOptions[m.cursor]
			currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
			m.config.ComputeChanged = (m.config.NumGPUs != currentNumGPUs)
		}
		m.step = modifyStepDiskSize
		m.cursor = 0
		m.diskInputTouched = false
		m.diskInput.Focus()
		return m, nil

	case modifyStepDiskSize:
		diskSize, err := strconv.Atoi(m.diskInput.Value())
		if err != nil || diskSize < 100 || diskSize > 1000 {
			m.validationErr = fmt.Errorf("disk size must be between 100 and 1000 GB")
			return m, nil
		}

		// Check against current instance size
		if diskSize < m.currentInstance.Storage {
			m.validationErr = fmt.Errorf("disk size cannot be smaller than current size (%d GB)", m.currentInstance.Storage)
			return m, nil
		}

		m.config.DiskSizeGB = diskSize
		m.config.DiskChanged = (diskSize != m.currentInstance.Storage)
		m.validationErr = nil

		// Check if any changes were made
		if !m.config.ModeChanged && !m.config.GPUChanged && !m.config.ComputeChanged && !m.config.DiskChanged {
			// No changes, exit with a special error
			m.err = fmt.Errorf("no changes")
			m.quitting = true
			return m, tea.Quit
		}

		m.step = modifyStepConfirmation
		m.cursor = 0
		m.diskInput.Blur()

	case modifyStepConfirmation:
		if m.cursor == 0 { // Apply Changes
			m.config.Confirmed = true
			m.step = modifyStepComplete
			m.quitting = true
			return m, tea.Quit
		} else { // Cancel
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m modifyModel) getCurrentGPUCursorPosition() int {
	effectiveMode := m.currentInstance.Mode
	if m.config.ModeChanged {
		effectiveMode = m.config.Mode
	}

	currentGPU := strings.ToLower(m.currentInstance.GPUType)

	if effectiveMode == "prototyping" {
		if currentGPU == "t4" {
			return 0
		}
		return 1 // a100xl
	} else {
		if currentGPU == "a100xl" || currentGPU == "a100" {
			return 0
		}
		return 1 // h100
	}
}

func (m modifyModel) formatGPUType(gpuType string) string {
	gpuType = strings.ToLower(gpuType)
	switch gpuType {
	case "t4":
		return "T4"
	case "a100xl", "a100":
		return "A100 80GB"
	case "h100":
		return "H100"
	default:
		return gpuType
	}
}

func (m modifyModel) getCurrentComputeCursorPosition() int {
	effectiveMode := m.currentInstance.Mode
	if m.config.ModeChanged {
		effectiveMode = m.config.Mode
	}

	if effectiveMode == "prototyping" {
		currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
		vcpuOptions := []int{4, 8, 16, 32}
		for i, vcpus := range vcpuOptions {
			if vcpus == currentVCPUs {
				return i
			}
		}
		return 0
	} else {
		currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
		gpuOptions := []int{1, 2, 4}
		for i, gpus := range gpuOptions {
			if gpus == currentNumGPUs {
				return i
			}
		}
		return 0
	}
}

func (m modifyModel) getMaxCursor() int {
	switch m.step {
	case modifyStepMode:
		return 1 // Prototyping, Production

	case modifyStepGPU:
		return 1 // 2 GPU options (t4/a100xl or a100xl/h100)

	case modifyStepCompute:
		effectiveMode := m.currentInstance.Mode
		if m.config.ModeChanged {
			effectiveMode = m.config.Mode
		}

		if effectiveMode == "prototyping" {
			return 3 // 4 vCPU options
		} else {
			return 2 // 3 GPU options
		}

	case modifyStepConfirmation:
		return 1 // Apply Changes, Cancel
	}

	return 0
}

func (m modifyModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	// Title
	s.WriteString(m.styles.title.Render("⚙ Modify Instance Configuration"))
	s.WriteString("\n\n")

	// Show current instance info
	s.WriteString(m.styles.label.Render(fmt.Sprintf("Instance: (%s) %s", m.currentInstance.ID, m.currentInstance.Name)))
	s.WriteString("\n\n")

	// Render current step
	switch m.step {
	case modifyStepMode:
		s.WriteString(m.renderModeStep())
	case modifyStepGPU:
		s.WriteString(m.renderGPUStep())
	case modifyStepCompute:
		s.WriteString(m.renderComputeStep())
	case modifyStepDiskSize:
		s.WriteString(m.renderDiskSizeStep())
	case modifyStepConfirmation:
		s.WriteString(m.renderConfirmationStep())
	}

	// Help text
	s.WriteString("\n")
	if m.step == modifyStepConfirmation {
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Confirm  Q: Cancel"))
	} else if m.step == modifyStepDiskSize {
		s.WriteString(m.styles.help.Render("Type disk size  Enter: Continue  ESC: Back  Q: Quit"))
	} else {
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  ESC: Back  Q: Quit"))
	}

	return s.String()
}

func (m modifyModel) renderModeStep() string {
	var s strings.Builder

	s.WriteString("Select instance mode:\n\n")

	modeLabels := []string{
		"Prototyping (lowest cost, dev/test)",
		"Production (highest stability, long-running)",
	}
	modeValues := []string{"prototyping", "production"}

	for i, label := range modeLabels {
		option := label
		if strings.EqualFold(modeValues[i], m.currentInstance.Mode) {
			option += " [current]"
		}

		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.cursor.Render("▶ ")
			option = m.styles.selected.Render(option)
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
	}

	return s.String()
}

func (m modifyModel) renderGPUStep() string {
	var s strings.Builder

	s.WriteString("Select GPU type:\n\n")

	effectiveMode := m.currentInstance.Mode
	if m.config.ModeChanged {
		effectiveMode = m.config.Mode
	}

	var optionLabels []string
	var optionValues []string

	if effectiveMode == "prototyping" {
		optionLabels = []string{
			"T4 (more affordable)",
			"A100 80GB (more powerful)",
		}
		optionValues = []string{"t4", "a100xl"}
	} else {
		optionLabels = []string{
			"A100 80GB",
			"H100",
		}
		optionValues = []string{"a100xl", "h100"}
	}

	for i, label := range optionLabels {
		option := label
		// Case-insensitive comparison for [current] marker
		if strings.EqualFold(optionValues[i], m.currentInstance.GPUType) {
			option += " [current]"
		}

		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.cursor.Render("▶ ")
			option = m.styles.selected.Render(option)
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
	}

	return s.String()
}

func (m modifyModel) renderComputeStep() string {
	var s strings.Builder

	effectiveMode := m.currentInstance.Mode
	if m.config.ModeChanged {
		effectiveMode = m.config.Mode
	}

	if effectiveMode == "prototyping" {
		s.WriteString("Select vCPU count (8GB RAM per vCPU):\n\n")

		currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
		vcpuOptions := []int{4, 8, 16, 32}
		for i, vcpus := range vcpuOptions {
			ram := vcpus * 8
			option := fmt.Sprintf("%d vCPUs (%d GB RAM)", vcpus, ram)

			if vcpus == currentVCPUs {
				option += " [current]"
			}

			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
				option = m.styles.selected.Render(option)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
		}
	} else { // production
		s.WriteString("Select number of GPUs (18 vCPUs per GPU, 144GB RAM per GPU):\n\n")

		currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
		gpuOptions := []int{1, 2, 4}
		for i, gpus := range gpuOptions {
			vcpus := gpus * 18
			ram := gpus * 144
			option := fmt.Sprintf("%d GPU(s) → %d vCPUs, %d GB RAM", gpus, vcpus, ram)

			if gpus == currentNumGPUs {
				option += " [current]"
			}

			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
				option = m.styles.selected.Render(option)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
		}
	}

	return s.String()
}

func (m modifyModel) renderDiskSizeStep() string {
	var s strings.Builder

	s.WriteString(fmt.Sprintf("Enter disk size (GB) [current: %d GB]:\n\n", m.currentInstance.Storage))
	s.WriteString(fmt.Sprintf("Range: %d-1000 GB (cannot be smaller than current)\n\n", m.currentInstance.Storage))
	s.WriteString(m.diskInput.View())
	s.WriteString("\n\n")

	if m.validationErr != nil {
		s.WriteString(errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v", m.validationErr)))
		s.WriteString("\n")
	}

	return s.String()
}

func (m modifyModel) renderConfirmationStep() string {
	var s strings.Builder

	s.WriteString("Review your configuration changes:\n\n")

	// Build change summary
	var changes []string

	if m.config.ModeChanged {
		changes = append(changes, fmt.Sprintf("Mode:      %s → %s", m.currentInstance.Mode, m.config.Mode))
	}

	if m.config.GPUChanged {
		currentGPU := m.formatGPUType(m.currentInstance.GPUType)
		newGPU := m.formatGPUType(m.config.GPUType)
		changes = append(changes, fmt.Sprintf("GPU Type:  %s → %s", currentGPU, newGPU))
	}

	if m.config.ComputeChanged {
		effectiveMode := m.currentInstance.Mode
		if m.config.ModeChanged {
			effectiveMode = m.config.Mode
		}

		if effectiveMode == "prototyping" {
			currentRAM, _ := strconv.Atoi(m.currentInstance.CPUCores)
			currentRAM *= 8
			newRAM := m.config.VCPUs * 8
			changes = append(changes, fmt.Sprintf("vCPUs:     %s → %d", m.currentInstance.CPUCores, m.config.VCPUs))
			changes = append(changes, fmt.Sprintf("RAM:       %d GB → %d GB", currentRAM, newRAM))
		} else {
			currentVCPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
			currentVCPUs *= 18
			newVCPUs := m.config.NumGPUs * 18
			currentRAM, _ := strconv.Atoi(m.currentInstance.NumGPUs)
			currentRAM *= 144
			newRAM := m.config.NumGPUs * 144
			changes = append(changes, fmt.Sprintf("GPUs:      %s → %d", m.currentInstance.NumGPUs, m.config.NumGPUs))
			changes = append(changes, fmt.Sprintf("vCPUs:     %d → %d", currentVCPUs, newVCPUs))
			changes = append(changes, fmt.Sprintf("RAM:       %d GB → %d GB", currentRAM, newRAM))
		}
	}

	if m.config.DiskChanged {
		changes = append(changes, fmt.Sprintf("Disk Size: %d GB → %d GB", m.currentInstance.Storage, m.config.DiskSizeGB))
	}

	if len(changes) == 0 {
		s.WriteString(warningStyleTUI.Render("⚠ Warning: No changes detected"))
		s.WriteString("\n\n")
	} else {
		// Display changes in a box
		changeBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(theme.PrimaryColor)).
			Padding(1, 2)

		changeText := "CHANGES:\n"
		for _, change := range changes {
			changeText += change + "\n"
		}

		s.WriteString(changeBox.Render(changeText))
		s.WriteString("\n\n")
	}

	s.WriteString(warningStyleTUI.Render("⚠ Warning: Modifying will restart the instance, running processes will be interrupted."))
	s.WriteString("\n\n")

	s.WriteString("Confirm modification?\n\n")

	options := []string{"✓ Apply Changes", "✗ Cancel"}
	for i, option := range options {
		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.cursor.Render("▶ ")
			option = m.styles.selected.Render(option)
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
	}

	return s.String()
}

// RunModifyInteractive starts the interactive modify flow
func RunModifyInteractive(client *api.Client, instance *api.Instance) (*ModifyConfig, error) {
	m := NewModifyModel(client, instance)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running interactive modify: %w", err)
	}

	finalModifyModel := finalModel.(modifyModel)

	if finalModifyModel.cancelled {
		return nil, &CancellationError{}
	}

	if finalModifyModel.err != nil {
		return nil, finalModifyModel.err
	}

	return &finalModifyModel.config, nil
}
