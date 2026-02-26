package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
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
	gpuCountPhase    bool // when true, modifyStepCompute shows GPU count selection before vCPU selection
	pricing          *PricingData
	pricingLoaded    bool

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

func NewModifyModel(client *api.Client, instance *api.Instance) tea.Model {
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

type modifyPricingMsg struct {
	rates map[string]float64
}

func fetchModifyPricingCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		rates, _ := client.FetchPricing()
		return modifyPricingMsg{rates: rates}
	}
}

func (m modifyModel) Init() tea.Cmd {
	return fetchModifyPricingCmd(m.client)
}

func (m modifyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case modifyPricingMsg:
		if msg.rates != nil {
			m.pricing = &PricingData{Rates: msg.rates}
		}
		m.pricingLoaded = true
		return m, nil

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
			if m.step == modifyStepCompute && !m.gpuCountPhase && m.needsGPUCountPhase() {
				// Go back to GPU count selection phase
				m.gpuCountPhase = true
				m.cursor = 0
				return m, nil
			}
			if m.step > modifyStepMode {
				m.step--
				m.cursor = 0
				m.gpuCountPhase = false
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
		effectiveMode := m.getEffectiveMode()

		var gpuValues []string
		if effectiveMode == "prototyping" {
			gpuValues = []string{"a6000", "a100xl", "h100"}
		} else {
			gpuValues = []string{"a100xl", "h100"}
		}

		m.config.GPUType = gpuValues[m.cursor]
		// Case-insensitive comparison
		m.config.GPUChanged = !strings.EqualFold(m.config.GPUType, m.currentInstance.GPUType)
		m.step = modifyStepCompute
		// H100 prototyping supports multi-GPU, so show GPU count selection first
		if m.needsGPUCountPhase() {
			m.gpuCountPhase = true
			m.cursor = m.getCurrentGPUCountCursorPosition()
		} else {
			m.gpuCountPhase = false
			if effectiveMode == "prototyping" {
				m.config.NumGPUs = 1
			}
			m.cursor = m.getCurrentComputeCursorPosition()
		}
		return m, nil

	case modifyStepCompute:
		effectiveMode := m.getEffectiveMode()

		if m.gpuCountPhase {
			// GPU count selection phase (H100 prototyping)
			gpuCounts := []int{1, 2}
			m.config.NumGPUs = gpuCounts[m.cursor]
			m.gpuCountPhase = false
			m.cursor = m.getCurrentComputeCursorPosition()
			// Stay on modifyStepCompute to show vCPU options next
			return m, nil
		}

		if effectiveMode == "prototyping" {
			vcpuOptions := m.getPrototypingVcpuOptions()
			m.config.VCPUs = vcpuOptions[m.cursor]
			currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
			currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
			m.config.ComputeChanged = (m.config.VCPUs != currentVCPUs) || (m.config.NumGPUs != currentNumGPUs)
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
		}
		// Cancel
		m.cancelled = true
		m.quitting = true
		return m, tea.Quit
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
		if currentGPU == "a6000" {
			return 0
		}
		if currentGPU == "a100xl" {
			return 1
		}
		return 2 // h100
	}
	if currentGPU == "a100xl" {
		return 0
	}
	return 1 // h100
}

func (m modifyModel) formatGPUType(gpuType string) string {
	return utils.FormatGPUType(gpuType)
}

func (m modifyModel) getEffectiveMode() string {
	if m.config.ModeChanged {
		return m.config.Mode
	}
	return m.currentInstance.Mode
}

func (m modifyModel) needsGPUCountPhase() bool {
	return m.getEffectiveMode() == "prototyping" && m.config.GPUType == "h100"
}

func (m modifyModel) getPrototypingVcpuOptions() []int {
	switch m.config.GPUType {
	case "a6000":
		return []int{4, 8}
	case "a100xl":
		return []int{4, 8, 12}
	case "h100":
		if m.config.NumGPUs == 2 {
			return []int{8, 12, 16, 20, 24}
		}
		return []int{4, 8, 12, 16}
	default:
		return []int{4, 8}
	}
}

func (m modifyModel) getCurrentGPUCountCursorPosition() int {
	currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
	gpuCounts := []int{1, 2}
	for i, count := range gpuCounts {
		if count == currentNumGPUs {
			return i
		}
	}
	return 0
}

func (m modifyModel) getCurrentComputeCursorPosition() int {
	effectiveMode := m.getEffectiveMode()

	if effectiveMode == "prototyping" {
		currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
		vcpuOptions := m.getPrototypingVcpuOptions()
		for i, vcpus := range vcpuOptions {
			if vcpus == currentVCPUs {
				return i
			}
		}
		return 0
	}
	currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
	gpuOptions := []int{1, 2, 4}
	for i, gpus := range gpuOptions {
		if gpus == currentNumGPUs {
			return i
		}
	}
	return 0
}

func (m modifyModel) getMaxCursor() int {
	switch m.step {
	case modifyStepMode:
		return 1 // Prototyping, Production

	case modifyStepGPU:
		if m.getEffectiveMode() == "prototyping" {
			return 2 // 3 GPU options (a6000/a100xl/h100)
		}
		return 1 // 2 GPU options (a100xl/h100)

	case modifyStepCompute:
		if m.gpuCountPhase {
			return 1 // 1 or 2 GPUs
		}
		if m.getEffectiveMode() == "prototyping" {
			return len(m.getPrototypingVcpuOptions()) - 1
		}
		return 2 // 3 production GPU options

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
	s.WriteString(m.styles.title.Render("Modify Instance Configuration"))
	s.WriteString("\n")

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

	// Pricing line
	if m.pricing != nil {
		price := m.computePreviewPrice()
		s.WriteString("\n")
		s.WriteString(m.styles.help.Render(fmt.Sprintf("Estimated cost: %s", FormatPrice(price))))
	}

	// Help text
	s.WriteString("\n")
	switch m.step {
	case modifyStepConfirmation:
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Confirm  Q: Cancel"))
	case modifyStepDiskSize:
		s.WriteString(m.styles.help.Render("Type disk size  Enter: Continue  ESC: Back  Q: Quit"))
	default:
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  ESC: Back  Q: Quit"))
	}

	return s.String()
}

// computePreviewPrice calculates the price for the resulting configuration,
// using current instance values as base and overriding with selections/hovered option.
func (m modifyModel) computePreviewPrice() float64 {
	// Start with current instance values
	mode := strings.ToLower(m.currentInstance.Mode)
	gpuType := strings.ToLower(m.currentInstance.GPUType)
	numGPUs := 1
	if n, err := strconv.Atoi(m.currentInstance.NumGPUs); err == nil {
		numGPUs = n
	}
	vcpus := 4
	if n, err := strconv.Atoi(m.currentInstance.CPUCores); err == nil {
		vcpus = n
	}
	diskSizeGB := m.currentInstance.Storage

	// Override with already-confirmed selections
	if m.config.ModeChanged {
		mode = m.config.Mode
	}
	if m.config.GPUChanged {
		gpuType = m.config.GPUType
	}
	if m.config.ComputeChanged {
		if m.config.NumGPUs > 0 {
			numGPUs = m.config.NumGPUs
		}
		if m.config.VCPUs > 0 {
			vcpus = m.config.VCPUs
		}
	}
	if m.config.DiskChanged {
		diskSizeGB = m.config.DiskSizeGB
	}

	// Override with hovered option for the current step
	switch m.step {
	case modifyStepMode:
		modeOptions := []string{"prototyping", "production"}
		mode = modeOptions[m.cursor]
	case modifyStepGPU:
		effectiveMode := m.getEffectiveMode()
		if effectiveMode == "prototyping" {
			gpuValues := []string{"a6000", "a100xl", "h100"}
			gpuType = gpuValues[m.cursor]
		} else {
			gpuValues := []string{"a100xl", "h100"}
			gpuType = gpuValues[m.cursor]
		}
	case modifyStepCompute:
		effectiveMode := m.getEffectiveMode()
		if m.gpuCountPhase {
			gpuCounts := []int{1, 2}
			numGPUs = gpuCounts[m.cursor]
			vcpus = includedVCPUs(gpuType, numGPUs)
		} else if effectiveMode == "prototyping" {
			vcpuOptions := m.getPrototypingVcpuOptions()
			vcpus = vcpuOptions[m.cursor]
		} else {
			gpuOptions := []int{1, 2, 4}
			numGPUs = gpuOptions[m.cursor]
		}
	case modifyStepDiskSize:
		if v, err := strconv.Atoi(m.diskInput.Value()); err == nil && v >= 100 {
			diskSizeGB = v
		}
	}

	// For production mode, set vcpus based on numGPUs (auto-calculated)
	if mode == "production" {
		vcpus = 18 * numGPUs
	}

	return CalculateHourlyPrice(m.pricing, mode, gpuType, numGPUs, vcpus, diskSizeGB)
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
			"RTX A6000 (more affordable)",
			"A100 80GB (high performance)",
			"H100 (most powerful)",
		}
		optionValues = []string{"a6000", "a100xl", "h100"}
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

	effectiveMode := m.getEffectiveMode()

	if m.gpuCountPhase {
		s.WriteString("Select number of GPUs:\n\n")

		currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
		gpuCounts := []int{1, 2}
		for i, num := range gpuCounts {
			option := fmt.Sprintf("%d GPU(s)", num)

			if num == currentNumGPUs {
				option += " [current]"
			}

			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
				option = m.styles.selected.Render(option)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
		}
	} else if effectiveMode == "prototyping" {
		s.WriteString("Select vCPU count (8GB RAM per vCPU):\n\n")

		currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
		vcpuOptions := m.getPrototypingVcpuOptions()
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

	s.WriteString("Review your configuration changes:\n")

	// Build change summary using panel style like create.go
	var panel strings.Builder

	if m.config.ModeChanged {
		panel.WriteString(m.styles.label.Render("Mode:       ") + fmt.Sprintf("%s → %s", utils.Capitalize(m.currentInstance.Mode), utils.Capitalize(m.config.Mode)) + "\n")
	}

	if m.config.GPUChanged {
		currentGPU := m.formatGPUType(m.currentInstance.GPUType)
		newGPU := m.formatGPUType(m.config.GPUType)
		panel.WriteString(m.styles.label.Render("GPU Type:   ") + fmt.Sprintf("%s → %s", currentGPU, newGPU) + "\n")
	}

	if m.config.ComputeChanged {
		effectiveMode := m.currentInstance.Mode
		if m.config.ModeChanged {
			effectiveMode = m.config.Mode
		}

		if effectiveMode == "prototyping" {
			currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
			if m.config.NumGPUs != currentNumGPUs {
				panel.WriteString(m.styles.label.Render("GPUs:       ") + fmt.Sprintf("%d → %d", currentNumGPUs, m.config.NumGPUs) + "\n")
			}
			currentRAM, _ := strconv.Atoi(m.currentInstance.CPUCores)
			currentRAM *= 8
			newRAM := m.config.VCPUs * 8
			panel.WriteString(m.styles.label.Render("vCPUs:      ") + fmt.Sprintf("%s → %d", m.currentInstance.CPUCores, m.config.VCPUs) + "\n")
			panel.WriteString(m.styles.label.Render("RAM:        ") + fmt.Sprintf("%d GB → %d GB", currentRAM, newRAM) + "\n")
		} else {
			currentVCPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
			currentVCPUs *= 18
			newVCPUs := m.config.NumGPUs * 18
			currentRAM, _ := strconv.Atoi(m.currentInstance.NumGPUs)
			currentRAM *= 144
			newRAM := m.config.NumGPUs * 144
			panel.WriteString(m.styles.label.Render("GPUs:       ") + fmt.Sprintf("%s → %d", m.currentInstance.NumGPUs, m.config.NumGPUs) + "\n")
			panel.WriteString(m.styles.label.Render("vCPUs:      ") + fmt.Sprintf("%d → %d", currentVCPUs, newVCPUs) + "\n")
			panel.WriteString(m.styles.label.Render("RAM:        ") + fmt.Sprintf("%d GB → %d GB", currentRAM, newRAM) + "\n")
		}
	}

	if m.config.DiskChanged {
		panel.WriteString(m.styles.label.Render("Disk Size:  ") + fmt.Sprintf("%d GB → %d GB", m.currentInstance.Storage, m.config.DiskSizeGB) + "\n")
	}

	panelStr := panel.String()
	if panelStr == "" {
		s.WriteString(warningStyleTUI.Render("⚠ Warning: No changes detected"))
		s.WriteString("\n\n")
	} else {
		// Trim trailing newline for consistent panel rendering
		panelStr = strings.TrimSuffix(panelStr, "\n")
		s.WriteString(m.styles.panel.Render(panelStr))
	}

	s.WriteString(warningStyleTUI.Render("⚠ Warning: Modifying will restart the instance, running processes will be interrupted."))
	s.WriteString("\n")

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

// RunModifyInstanceSelector shows an interactive instance selector for modify
func RunModifyInstanceSelector(client *api.Client, instances []api.Instance) (*api.Instance, error) {
	InitCommonStyles(os.Stdout)
	m := newModifyInstanceSelectorModel(instances)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running instance selector: %w", err)
	}

	result := finalModel.(modifyInstanceSelectorModel)

	if result.cancelled {
		return nil, &CancellationError{}
	}

	if result.selected == nil {
		return nil, &CancellationError{}
	}

	return result.selected, nil
}

type modifyInstanceSelectorModel struct {
	cursor    int
	instances []api.Instance
	selected  *api.Instance
	cancelled bool
	quitting  bool
	styles    modifyStyles
}

func newModifyInstanceSelectorModel(instances []api.Instance) modifyInstanceSelectorModel {
	return modifyInstanceSelectorModel{
		cursor:    0,
		instances: instances,
		styles:    newModifyStyles(),
	}
}

func (m modifyInstanceSelectorModel) Init() tea.Cmd {
	return nil
}

func (m modifyInstanceSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		case "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down":
			if m.cursor < len(m.instances)-1 {
				m.cursor++
			}

		case "enter":
			m.selected = &m.instances[m.cursor]
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m modifyInstanceSelectorModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString(m.styles.title.Render("⚙ Modify Thunder Compute Instance"))
	s.WriteString("\n")
	s.WriteString("Select an instance to modify:\n\n")

	for i, instance := range m.instances {
		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.cursor.Render("▶ ")
		}

		// Determine status style
		var statusStyle lipgloss.Style
		statusSuffix := ""
		switch instance.Status {
		case "RUNNING":
			statusStyle = SuccessStyle()
		case "STARTING":
			statusStyle = WarningStyle()
		case "DELETING":
			statusStyle = ErrorStyle()
			statusSuffix = " (deleting)"
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
			instance.GPUType,
			utils.Capitalize(instance.Mode),
		)

		s.WriteString(fmt.Sprintf("%s%s%s\n", cursor, idAndName, rest))
	}

	s.WriteString("\n")
	s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  Q: Cancel\n"))

	return s.String()
}
