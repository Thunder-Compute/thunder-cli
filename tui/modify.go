package tui

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Thunder-Compute/thunder-cli/api"
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

// ModifyPresets holds flag values provided on the command line for hybrid mode.
type ModifyPresets struct {
	Mode       *string
	GPUType    *string
	NumGPUs    *int
	VCPUs      *int
	DiskSizeGB *int
}

// IsEmpty returns true if no preset flags were set.
func (p *ModifyPresets) IsEmpty() bool {
	return p.Mode == nil && p.GPUType == nil && p.NumGPUs == nil &&
		p.VCPUs == nil && p.DiskSizeGB == nil
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
	pricing          *utils.PricingData
	pricingLoaded    bool
	specs            *utils.SpecStore
	presets          *ModifyPresets
	skippedSteps     map[modifyStep]bool

	styles PanelStyles
}

func NewModifyModel(client *api.Client, instance *api.Instance, specs *utils.SpecStore) tea.Model {
	styles := NewPanelStyles()

	ti := textinput.New()
	ti.Placeholder = fmt.Sprintf("%d", instance.Storage)
	ti.SetValue(fmt.Sprintf("%d", instance.Storage))
	ti.CharLimit = 4
	ti.Width = 20
	ti.Prompt = "▶ "

	m := modifyModel{
		step:             modifyStepGPU,
		cursor:           0,
		config:           ModifyConfig{},
		currentInstance:  instance,
		client:           client,
		diskInput:        ti,
		diskInputTouched: false,
		skippedSteps:     make(map[modifyStep]bool),
		styles:           styles,
		specs:            specs,
	}
	m.skippedSteps[modifyStepMode] = true

	m.cursor = m.getCurrentGPUCursorPosition()

	return m
}

// NewModifyModelWithPresets creates a modifyModel with pre-filled values from CLI flags.
func NewModifyModelWithPresets(client *api.Client, instance *api.Instance, specs *utils.SpecStore, presets *ModifyPresets) modifyModel {
	m := NewModifyModel(client, instance, specs).(modifyModel)
	m.presets = presets
	m.trySkipCurrentStep()
	return m
}

// trySkipCurrentStep loops forward through steps, auto-filling each one from
// presets if the preset value is valid given the current config state.
func (m *modifyModel) trySkipCurrentStep() {
	for {
		skipped := false
		m.skippedSteps[m.step] = false

		switch m.step {
		case modifyStepMode:
			if m.presets != nil && m.presets.Mode != nil {
				mode := utils.NormalizeModeInput(*m.presets.Mode)
				if mode == "prototyping" || mode == "production" {
					m.config.Mode = mode
					m.config.ModeChanged = !strings.EqualFold(mode, m.currentInstance.Mode)
				}
			}
			m.skippedSteps[modifyStepMode] = true
			skipped = true

		case modifyStepGPU:
			if m.presets != nil && m.presets.GPUType != nil {
				canonical := strings.ToLower(*m.presets.GPUType)
				if normalized, ok := m.specs.NormalizeGPUType(canonical); ok {
					canonical = normalized
				}
				if m.specs.IsGPUTypeAvailable(canonical) {
					m.config.GPUType = canonical
					m.config.GPUChanged = !strings.EqualFold(canonical, m.currentInstance.GPUType)
					m.skippedSteps[modifyStepGPU] = true
					skipped = true
				}
			}

		case modifyStepCompute:
			skipped = m.trySkipModifyCompute()

		case modifyStepDiskSize:
			if m.presets != nil && m.presets.DiskSizeGB != nil {
				v := *m.presets.DiskSizeGB
				m.diskInput.SetValue(fmt.Sprintf("%d", v))
				minDisk, maxDisk := m.specs.StorageRange(m.config.GPUType, m.config.NumGPUs)
				if v >= max(m.currentInstance.Storage, minDisk) && v <= maxDisk {
					m.config.DiskSizeGB = v
					m.config.DiskChanged = v != m.currentInstance.Storage
					m.skippedSteps[modifyStepDiskSize] = true
					skipped = true
				}
			}

		case modifyStepConfirmation:
			return
		}

		if !skipped {
			m.initModifyStep()
			return
		}

		m.step++
	}
}

// trySkipModifyCompute handles the compute step preset logic for modify.
func (m *modifyModel) trySkipModifyCompute() bool {
	if m.presets == nil {
		return false
	}

	effectiveGPU := m.config.GPUType
	if effectiveGPU == "" {
		effectiveGPU = strings.ToLower(m.currentInstance.GPUType)
	}

	needsCount := len(m.specs.GPUCounts(effectiveGPU)) > 1

	if !needsCount {
		gpuCounts := m.specs.GPUCounts(effectiveGPU)
		if len(gpuCounts) > 0 {
			m.config.NumGPUs = gpuCounts[0]
		} else {
			m.config.NumGPUs = 1
		}
		if !m.specs.IsSpecAvailable(effectiveGPU, m.config.NumGPUs) {
			return false
		}
		if m.presets.VCPUs == nil {
			return false
		}
		if slices.Contains(m.specs.VCPUOptions(effectiveGPU, m.config.NumGPUs), *m.presets.VCPUs) {
			m.config.VCPUs = *m.presets.VCPUs
			currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
			m.config.ComputeChanged = m.config.VCPUs != currentVCPUs
			return true
		}
		return false
	}

	// Multi-GPU: need both to fully skip
	if m.presets.NumGPUs != nil && m.presets.VCPUs != nil {
		if slices.Contains(m.specs.GPUCounts(effectiveGPU), *m.presets.NumGPUs) &&
			m.specs.IsSpecAvailable(effectiveGPU, *m.presets.NumGPUs) {
			vcpuOpts := m.specs.VCPUOptions(effectiveGPU, *m.presets.NumGPUs)
			if len(vcpuOpts) == 1 {
				m.config.NumGPUs = *m.presets.NumGPUs
				m.config.VCPUs = vcpuOpts[0]
				currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
				currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
				m.config.ComputeChanged = (m.config.VCPUs != currentVCPUs) || (m.config.NumGPUs != currentNumGPUs)
				return true
			}
			if slices.Contains(vcpuOpts, *m.presets.VCPUs) {
				m.config.NumGPUs = *m.presets.NumGPUs
				m.config.VCPUs = *m.presets.VCPUs
				currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
				currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
				m.config.ComputeChanged = (m.config.VCPUs != currentVCPUs) || (m.config.NumGPUs != currentNumGPUs)
				return true
			}
		}
		return false
	}

	// Only num-gpus provided
	if m.presets.NumGPUs != nil {
		if slices.Contains(m.specs.GPUCounts(effectiveGPU), *m.presets.NumGPUs) &&
			m.specs.IsSpecAvailable(effectiveGPU, *m.presets.NumGPUs) {
			m.config.NumGPUs = *m.presets.NumGPUs
			vcpuOpts := m.specs.VCPUOptions(effectiveGPU, *m.presets.NumGPUs)
			if len(vcpuOpts) == 1 {
				m.config.VCPUs = vcpuOpts[0]
				currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
				currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
				m.config.ComputeChanged = (m.config.VCPUs != currentVCPUs) || (m.config.NumGPUs != currentNumGPUs)
				return true
			}
			m.gpuCountPhase = false
			return false // don't skip whole step
		}
	}

	return false
}

// initModifyStep sets up step-specific state when arriving at a visible step.
func (m *modifyModel) initModifyStep() {
	m.cursor = 0
	switch m.step {
	case modifyStepGPU:
		m.cursor = m.getCurrentGPUCursorPosition()
	case modifyStepCompute:
		if m.needsGPUCountPhase() && m.config.NumGPUs == 0 {
			m.gpuCountPhase = true
			m.cursor = m.getCurrentGPUCountCursorPosition()
		} else if !m.needsGPUCountPhase() {
			gpuCounts := m.specs.GPUCounts(m.config.GPUType)
			if len(gpuCounts) > 0 {
				m.config.NumGPUs = gpuCounts[0]
			} else {
				m.config.NumGPUs = 1
			}
			m.gpuCountPhase = false
			m.cursor = m.getCurrentComputeCursorPosition()
		} else {
			m.cursor = m.getCurrentComputeCursorPosition()
		}
	case modifyStepDiskSize:
		m.diskInput.Focus()
		m.diskInputTouched = false
	default:
		m.diskInput.Blur()
	}
}

func (m *modifyModel) resetComputeSelection() {
	m.config.NumGPUs = 0
	m.config.VCPUs = 0
	m.config.ComputeChanged = false
	m.gpuCountPhase = false
}

// prevVisibleStep returns the previous non-skipped step. Returns -1 if none.
func (m *modifyModel) prevVisibleStep(from modifyStep) modifyStep {
	for s := from - 1; s >= modifyStepMode; s-- {
		if !m.skippedSteps[s] {
			return s
		}
	}
	return -1
}

type modifyPricingMsg struct {
	rates map[string]float64
	err   error
}

func fetchModifyPricingCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		rates, err := client.FetchPricing()
		return modifyPricingMsg{rates: rates, err: err}
	}
}

func (m modifyModel) Init() tea.Cmd {
	return fetchModifyPricingCmd(m.client)
}

func (m modifyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case modifyPricingMsg:
		if msg.err == nil && msg.rates != nil {
			m.pricing = &utils.PricingData{Rates: msg.rates}
		}
		m.pricingLoaded = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		case "q", "Q":
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
			prev := m.prevVisibleStep(m.step)
			if prev < 0 {
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			}
			m.step = prev
			m.cursor = 0
			m.gpuCountPhase = false
			m.validationErr = nil
			if m.step == modifyStepDiskSize {
				m.diskInput.Focus()
				m.diskInputTouched = false
			} else {
				m.diskInput.Blur()
			}
			return m, nil

		case "up", "k":
			if m.step != modifyStepDiskSize {
				if m.cursor > 0 {
					m.cursor--
				}
			}

		case "down", "j":
			if m.step != modifyStepDiskSize {
				maxCursor := m.getMaxCursor()
				if m.cursor < maxCursor {
					m.cursor++
				}
			}

		case "enter":
			return m.handleEnter()
		}

		// Handle text input for disk size steps
		if m.step == modifyStepDiskSize {
			if len(msg.String()) == 1 && msg.Type == tea.KeyRunes {
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
		m.config.ModeChanged = !strings.EqualFold(newMode, m.currentInstance.Mode)
		m.config.GPUType = ""
		m.config.GPUChanged = false
		m.resetComputeSelection()
		m.step = modifyStepGPU
		m.trySkipCurrentStep()
		// If we're on the GPU step after skipping, set cursor to current GPU position
		if m.step == modifyStepGPU {
			m.cursor = m.getCurrentGPUCursorPosition()
		}
		if m.quitting {
			return m, tea.Quit
		}
		return m, nil

	case modifyStepGPU:
		gpuValues := m.specs.GPUOptions()
		if !m.specs.IsGPUTypeAvailable(gpuValues[m.cursor]) {
			return m, nil
		}
		m.resetComputeSelection()
		m.config.GPUType = gpuValues[m.cursor]
		m.config.GPUChanged = !strings.EqualFold(m.config.GPUType, m.currentInstance.GPUType)
		m.step = modifyStepCompute
		m.trySkipCurrentStep()
		if m.quitting {
			return m, tea.Quit
		}
		return m, nil

	case modifyStepCompute:
		if m.gpuCountPhase {
			gpuCounts := m.specs.GPUCounts(m.config.GPUType)
			if !m.specs.IsSpecAvailable(m.config.GPUType, gpuCounts[m.cursor]) {
				return m, nil
			}
			m.config.NumGPUs = gpuCounts[m.cursor]
			m.gpuCountPhase = false
			// Check if vCPUs preset can now be applied
			if m.presets != nil && m.presets.VCPUs != nil {
				if slices.Contains(m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs), *m.presets.VCPUs) {
					m.config.VCPUs = *m.presets.VCPUs
					currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
					currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
					m.config.ComputeChanged = (m.config.VCPUs != currentVCPUs) || (m.config.NumGPUs != currentNumGPUs)
					m.step = modifyStepDiskSize
					m.trySkipCurrentStep()
					if m.quitting {
						return m, tea.Quit
					}
					return m, nil
				}
			}
			m.cursor = m.getCurrentComputeCursorPosition()
			return m, nil
		}

		vcpuOptions := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs)
		m.config.VCPUs = vcpuOptions[m.cursor]
		currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
		currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
		m.config.ComputeChanged = (m.config.VCPUs != currentVCPUs) || (m.config.NumGPUs != currentNumGPUs)
		m.step = modifyStepDiskSize
		m.trySkipCurrentStep()
		if m.quitting {
			return m, tea.Quit
		}
		return m, nil

	case modifyStepDiskSize:
		minDisk, maxDisk := m.specs.StorageRange(m.config.GPUType, m.config.NumGPUs)
		diskSize, err := strconv.Atoi(m.diskInput.Value())
		if err != nil || diskSize < minDisk || diskSize > maxDisk {
			m.validationErr = fmt.Errorf("primary storage for this instance type must be between %d and %d GB. Check storage limits at https://www.thundercompute.com/pricing", minDisk, maxDisk)
			return m, nil
		}

		if diskSize < m.currentInstance.Storage {
			m.validationErr = fmt.Errorf("primary storage cannot be smaller than current size (%d GB)", m.currentInstance.Storage)
			return m, nil
		}

		m.config.DiskSizeGB = diskSize
		m.config.DiskChanged = (diskSize != m.currentInstance.Storage)
		m.validationErr = nil

		if !m.config.ModeChanged && !m.config.GPUChanged && !m.config.ComputeChanged && !m.config.DiskChanged {
			m.err = ErrNoChanges
			m.quitting = true
			return m, tea.Quit
		}

		m.diskInput.Blur()
		m.step = modifyStepConfirmation
		m.trySkipCurrentStep()
		if m.quitting {
			return m, tea.Quit
		}
		return m, nil

	case modifyStepConfirmation:
		if m.cursor == 0 {
			m.config.Confirmed = true
			m.step = modifyStepComplete
			m.quitting = true
			return m, tea.Quit
		}
		m.cancelled = true
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m modifyModel) getCurrentGPUCursorPosition() int {
	currentGPU := strings.ToLower(m.currentInstance.GPUType)
	gpuOptions := m.specs.GPUOptions()
	for i, gpu := range gpuOptions {
		if gpu == currentGPU {
			return i
		}
	}
	return 0
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
	return m.specs.NeedsGPUCountPhase(m.config.GPUType)
}

func (m modifyModel) getCurrentGPUCountCursorPosition() int {
	currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
	gpuCounts := m.specs.GPUCounts(m.config.GPUType)
	for i, count := range gpuCounts {
		if count == currentNumGPUs {
			return i
		}
	}
	return 0
}

func (m modifyModel) getCurrentComputeCursorPosition() int {
	currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
	vcpuOptions := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs)
	for i, vcpus := range vcpuOptions {
		if vcpus == currentVCPUs {
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
		return len(m.specs.GPUOptions()) - 1

	case modifyStepCompute:
		if m.gpuCountPhase {
			return len(m.specs.GPUCounts(m.config.GPUType)) - 1
		}
		return len(m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs)) - 1

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
	s.WriteString(m.styles.Title.Render("Modify Instance Configuration"))
	s.WriteString("\n")

	// Show current instance info
	s.WriteString(m.styles.Label.Render(fmt.Sprintf("Instance: (%s) %s", m.currentInstance.ID, m.currentInstance.Name)))
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

	// Pricing line (skip on mode step since config is too incomplete)
	if m.pricing != nil && m.step != modifyStepMode {
		price := m.computePreviewPrice()
		s.WriteString("\n")
		s.WriteString(m.styles.Help.Render(fmt.Sprintf("Estimated cost: %s", utils.FormatPrice(price))))
	}

	// Help text
	s.WriteString("\n")
	switch m.step {
	case modifyStepConfirmation:
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Confirm  Esc: Back  Q: Quit"))
	case modifyStepDiskSize:
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Quit"))
	default:
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Quit"))
	}

	return s.String()
}

// computePreviewPrice calculates the price for the resulting configuration,
// using current instance values as base and overriding with selections/hovered option.
func (m modifyModel) computePreviewPrice() float64 {
	// Start with current instance values
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
	case modifyStepGPU:
		gpuValues := m.specs.GPUOptions()
		gpuType = gpuValues[m.cursor]
		gpuCounts := m.specs.GPUCounts(gpuType)
		if !slices.Contains(gpuCounts, numGPUs) && len(gpuCounts) > 0 {
			numGPUs = gpuCounts[0]
			vcpus = m.specs.IncludedVCPUs(gpuType, numGPUs)
		}
	case modifyStepCompute:
		if m.gpuCountPhase {
			gpuCounts := m.specs.GPUCounts(m.config.GPUType)
			numGPUs = gpuCounts[m.cursor]
			vcpus = m.specs.IncludedVCPUs(gpuType, numGPUs)
		} else {
			vcpuOptions := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs)
			vcpus = vcpuOptions[m.cursor]
		}
	case modifyStepDiskSize:
		if v, err := strconv.Atoi(m.diskInput.Value()); err == nil && v >= 100 {
			diskSizeGB = v
		}
	}

	included := m.specs.IncludedVCPUs(gpuType, numGPUs)
	return utils.CalculateHourlyPrice(m.pricing, "", gpuType, numGPUs, vcpus, diskSizeGB, included)
}

func (m modifyModel) renderModeStep() string {
	var s strings.Builder

	s.WriteString("Select configuration profile:\n\n")

	modeLabels := []string{
		"Default",
		"Large GPU count",
	}
	modeValues := []string{"prototyping", "production"}

	for i, label := range modeLabels {
		option := label
		if strings.EqualFold(modeValues[i], m.currentInstance.Mode) {
			option += " [current]"
		}

		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.Cursor.Render("▶ ")
			option = m.styles.Selected.Render(option)
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
	}

	return s.String()
}

func (m modifyModel) renderGPUStep() string {
	var s strings.Builder

	s.WriteString("Select GPU type:\n\n")

	optionValues := m.specs.GPUOptions()
	optionLabels := make([]string, len(optionValues))
	for i, gpu := range optionValues {
		optionLabels[i] = utils.FormatGPUType(gpu)
	}

	for i, label := range optionLabels {
		option := label
		// Case-insensitive comparison for [current] marker
		if strings.EqualFold(optionValues[i], m.currentInstance.GPUType) {
			option += " [current]"
		}

		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.Cursor.Render("▶ ")
		}
		if !m.specs.IsGPUTypeAvailable(optionValues[i]) {
			option = subtleTextStyle.Render(option + " (unavailable)")
		} else if m.cursor == i {
			option = m.styles.Selected.Render(option)
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
	}
	if len(optionValues) > 0 && !m.specs.IsGPUTypeAvailable(optionValues[m.cursor]) {
		s.WriteString("\n")
		s.WriteString(warningStyleTUI.Render("This GPU type is currently unavailable. Choose another GPU."))
		s.WriteString("\n")
	}

	return s.String()
}

func (m modifyModel) renderComputeStep() string {
	var s strings.Builder

	if m.gpuCountPhase {
		s.WriteString("Select number of GPUs:\n\n")

		currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
		gpuCounts := m.specs.GPUCounts(m.config.GPUType)
		for i, num := range gpuCounts {
			option := fmt.Sprintf("%d GPU(s)", num)

			if num == currentNumGPUs {
				option += " [current]"
			}

			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.Cursor.Render("▶ ")
			}
			if !m.specs.IsSpecAvailable(m.config.GPUType, num) {
				option = subtleTextStyle.Render(option + " (unavailable)")
			} else if m.cursor == i {
				option = m.styles.Selected.Render(option)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
		}
		if len(gpuCounts) > 0 && !m.specs.IsSpecAvailable(m.config.GPUType, gpuCounts[m.cursor]) {
			s.WriteString("\n")
			s.WriteString(warningStyleTUI.Render("This GPU count is currently unavailable. Choose another count."))
			s.WriteString("\n")
		}
	} else {
		ramPerVCPU := m.specs.RamPerVCPU(m.config.GPUType, m.config.NumGPUs)
		s.WriteString(fmt.Sprintf("Select vCPU count (%dGB RAM per vCPU):\n\n", ramPerVCPU))

		currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
		vcpuOptions := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs)
		for i, vcpus := range vcpuOptions {
			ram := vcpus * ramPerVCPU
			option := fmt.Sprintf("%d vCPUs (%d GB RAM)", vcpus, ram)

			if vcpus == currentVCPUs {
				option += " [current]"
			}

			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.Cursor.Render("▶ ")
				option = m.styles.Selected.Render(option)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
		}
	}

	return s.String()
}

func (m modifyModel) renderDiskSizeStep() string {
	var s strings.Builder

	s.WriteString("Configure storage:\n\n")
	s.WriteString(m.styles.Selected.Render("▶ Primary") + fmt.Sprintf(" [current: %d GB]\n", m.currentInstance.Storage))
	s.WriteString("  " + m.diskInput.View() + "\n")

	if m.validationErr != nil {
		s.WriteString("\n")
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

	if m.config.GPUChanged {
		currentGPU := m.formatGPUType(m.currentInstance.GPUType)
		newGPU := m.formatGPUType(m.config.GPUType)
		panel.WriteString(m.styles.Label.Render("GPU Type:   ") + fmt.Sprintf("%s → %s", currentGPU, newGPU) + "\n")
	}

	if m.config.ComputeChanged {
		currentNumGPUs, _ := strconv.Atoi(m.currentInstance.NumGPUs)
		if m.config.NumGPUs != currentNumGPUs {
			panel.WriteString(m.styles.Label.Render("GPUs:       ") + fmt.Sprintf("%d → %d", currentNumGPUs, m.config.NumGPUs) + "\n")
		}
		currentRamPerVCPU := m.specs.RamPerVCPU(strings.ToLower(m.currentInstance.GPUType), currentNumGPUs)
		newRamPerVCPU := m.specs.RamPerVCPU(m.config.GPUType, m.config.NumGPUs)
		currentVCPUs, _ := strconv.Atoi(m.currentInstance.CPUCores)
		currentRAM := currentVCPUs * currentRamPerVCPU
		newRAM := m.config.VCPUs * newRamPerVCPU
		panel.WriteString(m.styles.Label.Render("vCPUs:      ") + fmt.Sprintf("%s → %d", m.currentInstance.CPUCores, m.config.VCPUs) + "\n")
		panel.WriteString(m.styles.Label.Render("RAM:        ") + fmt.Sprintf("%d GB → %d GB", currentRAM, newRAM) + "\n")
	}

	if m.config.DiskChanged {
		panel.WriteString(m.styles.Label.Render("Disk Size:  ") + fmt.Sprintf("%d GB → %d GB", m.currentInstance.Storage, m.config.DiskSizeGB) + "\n")
	}

	panelStr := panel.String()
	if panelStr == "" {
		s.WriteString(warningStyleTUI.Render("⚠ Warning: No changes detected"))
		s.WriteString("\n\n")
	} else {
		// Trim trailing newline for consistent panel rendering
		panelStr = strings.TrimSuffix(panelStr, "\n")
		s.WriteString(m.styles.Panel.Render(panelStr))
	}

	s.WriteString(warningStyleTUI.Render("⚠ Warning: Modifying will restart the instance, running processes will be interrupted."))
	s.WriteString("\n")

	s.WriteString("Confirm modification?\n\n")

	options := []string{"✓ Apply Changes", "✗ Cancel"}
	for i, option := range options {
		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.Cursor.Render("▶ ")
			option = m.styles.Selected.Render(option)
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
	}

	return s.String()
}

func runModifyModel(m tea.Model) (*ModifyConfig, error) {
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running interactive modify: %w", err)
	}

	finalModifyModel, ok := finalModel.(modifyModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if finalModifyModel.cancelled {
		return nil, ErrCancelled
	}

	if finalModifyModel.err != nil {
		return nil, finalModifyModel.err
	}

	return &finalModifyModel.config, nil
}

// RunModifyInteractive starts the interactive modify flow
func RunModifyInteractive(client *api.Client, instance *api.Instance, specs *utils.SpecStore) (*ModifyConfig, error) {
	m := NewModifyModel(client, instance, specs)
	return runModifyModel(m)
}

// RunModifyHybrid runs the modify TUI with some steps pre-filled from CLI flags.
func RunModifyHybrid(client *api.Client, instance *api.Instance, specs *utils.SpecStore, presets *ModifyPresets) (*ModifyConfig, error) {
	m := NewModifyModelWithPresets(client, instance, specs, presets)
	return runModifyModel(m)
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

	result, ok := finalModel.(modifyInstanceSelectorModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if result.cancelled {
		return nil, ErrCancelled
	}

	if result.selected == nil {
		return nil, ErrCancelled
	}

	return result.selected, nil
}

type modifyInstanceSelectorModel struct {
	cursor    int
	instances []api.Instance
	selected  *api.Instance
	cancelled bool
	quitting  bool
	styles    PanelStyles
}

func newModifyInstanceSelectorModel(instances []api.Instance) modifyInstanceSelectorModel {
	return modifyInstanceSelectorModel{
		cursor:    0,
		instances: instances,
		styles:    NewPanelStyles(),
	}
}

func (m modifyInstanceSelectorModel) Init() tea.Cmd {
	return nil
}

func (m modifyInstanceSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q", "esc":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
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

	s.WriteString(m.styles.Title.Render("⚙ Modify Thunder Compute Instance"))
	s.WriteString("\n")
	s.WriteString("Select an instance to modify:\n\n")

	for i, instance := range m.instances {
		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.Cursor.Render("▶ ")
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
			idAndName = m.styles.Selected.Render(idAndName)
		}

		statusText := statusStyle.Render(fmt.Sprintf("(%s)", instance.Status))
		rest := fmt.Sprintf(" %s%s - %sx%s",
			statusText,
			statusSuffix,
			instance.NumGPUs,
			instance.GPUType,
		)

		s.WriteString(fmt.Sprintf("%s%s%s\n", cursor, idAndName, rest))
	}

	s.WriteString("\n")
	s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Select  Esc/Q: Quit\n"))

	return s.String()
}
