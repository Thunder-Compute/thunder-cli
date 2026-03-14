package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
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
	step             createStep
	cursor           int
	config           CreateConfig
	templates        []api.TemplateEntry
	snapshots        []api.Snapshot
	templatesLoaded  bool
	snapshotsLoaded  bool
	diskInput        textinput.Model
	err              error
	validationErr    error
	quitting         bool
	client           *api.Client
	spinner          spinner.Model
	selectedSnapshot *api.Snapshot
	gpuCountPhase    bool // when true, stepCompute shows GPU count selection before vCPU selection
	pricing          *utils.PricingData
	pricingLoaded    bool

	styles PanelStyles
}

func NewCreateModel(client *api.Client) createModel {
	styles := NewPanelStyles()

	ti := textinput.New()
	ti.Placeholder = "100"
	ti.CharLimit = 4
	ti.Width = 20
	ti.Prompt = "▶ "
	ti.PromptStyle = styles.Cursor
	ti.TextStyle = styles.Cursor
	ti.PlaceholderStyle = styles.Cursor
	ti.Cursor.Style = styles.Cursor

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
	templates []api.TemplateEntry
	err       error
}

type createSnapshotsMsg struct {
	snapshots []api.Snapshot
	err       error
}

type createPricingMsg struct {
	rates map[string]float64
	err   error
}

func sortTemplates(templates []api.TemplateEntry) []api.TemplateEntry {
	sorted := make([]api.TemplateEntry, 0, len(templates))

	for _, t := range templates {
		if strings.EqualFold(t.Key, "base") {
			sorted = append(sorted, t)
			break
		}
	}

	for _, t := range templates {
		if !strings.EqualFold(t.Key, "base") {
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

func fetchCreatePricingCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		rates, err := client.FetchPricing()
		return createPricingMsg{rates: rates, err: err}
	}
}

func (m createModel) Init() tea.Cmd {
	return tea.Batch(fetchCreateTemplatesCmd(m.client), fetchCreateSnapshotsCmd(m.client), fetchCreatePricingCmd(m.client), m.spinner.Tick)
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

	case createPricingMsg:
		if msg.err == nil && msg.rates != nil {
			m.pricing = &utils.PricingData{Rates: msg.rates}
		}
		m.pricingLoaded = true
		return m, nil

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
			if m.step == stepCompute && !m.gpuCountPhase && m.config.Mode == "prototyping" && utils.NeedsGPUCountPhase(m.config.GPUType) {
				// Go back to GPU count selection phase
				m.gpuCountPhase = true
				m.cursor = 0
			} else if m.step > stepMode {
				m.step--
				m.cursor = 0
				m.gpuCountPhase = false
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
		default:
			// Only update text input for non-handled keys
			if m.step == stepDiskSize {
				var cmd tea.Cmd
				m.diskInput, cmd = m.diskInput.Update(msg)
				// Clear validation error when user modifies input (but not on enter)
				m.validationErr = nil
				return m, cmd
			}
		}
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
		// Some prototyping GPU types support multi-GPU, so show GPU count selection first
		if m.config.Mode == "prototyping" && utils.NeedsGPUCountPhase(m.config.GPUType) {
			m.gpuCountPhase = true
		} else if m.config.Mode == "prototyping" {
			m.config.NumGPUs = 1
			m.gpuCountPhase = false
		}

	case stepCompute:
		if m.gpuCountPhase {
			gpuCounts := utils.PrototypingGPUCounts(m.config.GPUType)
			m.config.NumGPUs = gpuCounts[m.cursor]
			m.gpuCountPhase = false
			m.cursor = 0
			// Stay on stepCompute to show vCPU options next
		} else if m.config.Mode == "prototyping" {
			vcpuOpts := utils.PrototypingVCPUOptions(m.config.GPUType, m.config.NumGPUs)
			m.config.VCPUs = vcpuOpts[m.cursor]
			m.step = stepTemplate
			m.cursor = 0
		} else {
			numGPUs := []int{1, 2, 4, 8}
			m.config.NumGPUs = numGPUs[m.cursor]
			m.config.VCPUs = 18 * m.config.NumGPUs
			m.step = stepTemplate
			m.cursor = 0
		}

	case stepTemplate:
		totalOptions := len(m.templates) + len(m.snapshots)
		if m.cursor < totalOptions {
			// Check if cursor is on a template or snapshot
			if m.cursor < len(m.templates) {
				// Selected a template
				m.config.Template = m.templates[m.cursor].Key
				m.selectedSnapshot = nil
				m.diskInput.SetValue("100")
			} else {
				// Selected a snapshot
				snapshotIndex := m.cursor - len(m.templates)
				snapshot := m.snapshots[snapshotIndex]
				m.config.Template = snapshot.Name
				m.selectedSnapshot = &snapshot
				// Set minimum disk size from snapshot
				m.diskInput.SetValue(fmt.Sprintf("%d", snapshot.MinimumDiskSizeGB))
			}
			m.step = stepDiskSize
			m.diskInput.Focus()
		}

	case stepDiskSize:
		diskSize, err := strconv.Atoi(m.diskInput.Value())
		if err != nil || diskSize < 100 || diskSize > 1000 {
			m.validationErr = fmt.Errorf("disk size must be between 100 and 1000 GB")
			return m, nil
		}
		// Check against snapshot minimum if a snapshot was selected
		if m.selectedSnapshot != nil && diskSize < m.selectedSnapshot.MinimumDiskSizeGB {
			m.validationErr = fmt.Errorf("disk size must be at least %d GB for snapshot '%s'", m.selectedSnapshot.MinimumDiskSizeGB, m.selectedSnapshot.Name)
			return m, nil
		}
		m.config.DiskSizeGB = diskSize
		m.validationErr = nil
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
	switch m.config.Mode {
	case "prototyping":
		return utils.PrototypingGPUOptions()
	case "production":
		return []string{"a100xl", "h100"}
	default:
		panic("Unknown config mode")
	}
}

func (m createModel) getMaxCursor() int {
	switch m.step {
	case stepMode:
		return 1
	case stepGPU:
		return len(m.getGPUOptions()) - 1
	case stepCompute:
		if m.gpuCountPhase {
			return len(utils.PrototypingGPUCounts(m.config.GPUType)) - 1
		}
		if m.config.Mode == "prototyping" {
			return len(utils.PrototypingVCPUOptions(m.config.GPUType, m.config.NumGPUs)) - 1
		}
		return 3
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
	s.WriteString(m.styles.Title.Render("⚡ Create Thunder Compute Instance"))
	s.WriteString("\n")

	progressSteps := []string{"Mode", "GPU", "Size", "Template", "Disk", "Confirm"}
	progress := ""
	for i, stepName := range progressSteps {
		adjustedStep := int(m.step)
		if i == adjustedStep {
			progress += m.styles.Selected.Render(fmt.Sprintf("[%s]", stepName))
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
				cursor = m.styles.Cursor.Render("▶ ")
			}
			display := mode
			if m.cursor == i {
				display = m.styles.Selected.Render(mode)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, display))
		}

	case stepGPU:
		s.WriteString("Select GPU type:\n\n")
		gpus := m.getGPUOptions()
		for i, gpu := range gpus {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.Cursor.Render("▶ ")
			}
			displayName := utils.FormatGPUType(gpu)

			switch gpu {
			case "a100xl":
				if m.config.Mode == "prototyping" {
					displayName = "A100 80GB (more powerful)"
				}
			case "h100":
				if m.config.Mode == "prototyping" {
					displayName += " (most powerful)"
				}
			case "a6000":
				if m.config.Mode == "prototyping" {
					displayName += " (more affordable)"
				}
			}
			if m.cursor == i {
				displayName = m.styles.Selected.Render(displayName)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, displayName))
		}

	case stepCompute:
		if m.gpuCountPhase {
			s.WriteString("Select number of GPUs:\n\n")
			gpuCounts := utils.PrototypingGPUCounts(m.config.GPUType)
			for i, num := range gpuCounts {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.Cursor.Render("▶ ")
				}
				text := fmt.Sprintf("%d GPU(s)", num)
				if m.cursor == i {
					text = m.styles.Selected.Render(text)
				}
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, text))
			}
		} else if m.config.Mode == "prototyping" {
			s.WriteString("Select vCPU count (8GB RAM per vCPU):\n\n")
			vcpuOpts := utils.PrototypingVCPUOptions(m.config.GPUType, m.config.NumGPUs)
			for i, vcpu := range vcpuOpts {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.Cursor.Render("▶ ")
				}
				ram := vcpu * 8
				line := fmt.Sprintf("%s%d vCPUs (%d GB RAM)", cursor, vcpu, ram)
				if m.cursor == i {
					line = fmt.Sprintf("%s%s", cursor, m.styles.Selected.Render(fmt.Sprintf("%d vCPUs (%d GB RAM)", vcpu, ram)))
				}
				s.WriteString(line + "\n")
			}
		} else {
			s.WriteString("Select number of GPUs (18 vCPUs per GPU, 90GB RAM per GPU):\n\n")
			numGPUs := []int{1, 2, 4, 8}
			for i, num := range numGPUs {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.Cursor.Render("▶ ")
				}
				vcpus := num * 18
				text := fmt.Sprintf("%d GPU(s) → %d vCPUs", num, vcpus)
				if m.cursor == i {
					text = m.styles.Selected.Render(text)
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
			s.WriteString(m.styles.Label.Render("Templates:") + "\n")
			for i, entry := range m.templates {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.Cursor.Render("▶ ")
				}
				name := entry.Template.DisplayName
				if entry.Template.ExtendedDescription != "" {
					name += fmt.Sprintf(" - %s", entry.Template.ExtendedDescription)
				}
				if m.cursor == i {
					name = m.styles.Selected.Render(name)
				}
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
			}

			// Display snapshots if any
			if len(m.snapshots) > 0 {
				s.WriteString("\n")
				s.WriteString(m.styles.Label.Render("Custom Snapshots:") + "\n")
				for i, snapshot := range m.snapshots {
					cursorIndex := len(m.templates) + i
					cursor := "  "
					if m.cursor == cursorIndex {
						cursor = m.styles.Cursor.Render("▶ ")
					}
					name := fmt.Sprintf("%s (%d GB)", snapshot.Name, snapshot.MinimumDiskSizeGB)
					if m.cursor == cursorIndex {
						name = m.styles.Selected.Render(name)
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
		if m.validationErr != nil {
			s.WriteString(errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v", m.validationErr)))
			s.WriteString("\n")
		}
		s.WriteString(m.styles.Help.Render("Press Enter to continue\n"))

	case stepConfirmation:
		s.WriteString("Review your configuration:\n")

		var panel strings.Builder
		panel.WriteString(m.styles.Label.Render("Mode:       ") + utils.Capitalize(m.config.Mode) + "\n")
		panel.WriteString(m.styles.Label.Render("GPU Type:   ") + utils.FormatGPUType(m.config.GPUType) + "\n")
		panel.WriteString(m.styles.Label.Render("GPUs:       ") + strconv.Itoa(m.config.NumGPUs) + "\n")
		panel.WriteString(m.styles.Label.Render("vCPUs:      ") + strconv.Itoa(m.config.VCPUs) + "\n")
		ramPerVCPU := 8
		if m.config.Mode == "production" {
			ramPerVCPU = 5
		}
		panel.WriteString(m.styles.Label.Render("RAM:        ") + strconv.Itoa(m.config.VCPUs*ramPerVCPU) + " GB\n")
		panel.WriteString(m.styles.Label.Render("Template:   ") + utils.Capitalize(m.config.Template) + "\n")
		panel.WriteString(m.styles.Label.Render("Disk Size:  ") + strconv.Itoa(m.config.DiskSizeGB) + " GB")

		s.WriteString(m.styles.Panel.Render(panel.String()))
		s.WriteString("\n")

		if m.config.Mode == "prototyping" {
			warning := "⚠ Prototyping mode is optimized for dev/testing; switch to production mode for inference servers or large training runs.\n"
			s.WriteString(warningStyleTUI.Render(warning))
			s.WriteString("\n")
		}

		s.WriteString("Confirm creation?\n\n")
		options := []string{"✓ Create Instance", "✗ Cancel"}

		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.Cursor.Render("▶ ")
			}
			text := option
			if m.cursor == i {
				text = m.styles.Selected.Render(option)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, text))
		}
	}

	// Pricing line (skip on mode step since config is too incomplete)
	if m.pricing != nil && m.step != stepMode {
		price := m.computePreviewPrice()
		s.WriteString("\n")
		s.WriteString(m.styles.Help.Render(fmt.Sprintf("Estimated cost: %s", utils.FormatPrice(price))))
	}

	if m.step != stepConfirmation {
		s.WriteString("\n")
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Cancel\n"))
	} else {
		s.WriteString("\n")
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Confirm  Q: Cancel\n"))
	}

	return s.String()
}

// computePreviewPrice calculates the price based on current config state,
// using the hovered option for the current step to preview pricing.
func (m createModel) computePreviewPrice() float64 {
	mode := m.config.Mode
	gpuType := m.config.GPUType
	numGPUs := m.config.NumGPUs
	vcpus := m.config.VCPUs
	diskSizeGB := m.config.DiskSizeGB

	// Apply defaults for unfilled fields
	if mode == "" {
		mode = "prototyping"
	}
	if gpuType == "" {
		if mode == "prototyping" {
			gpuType = "a6000"
		} else {
			gpuType = "a100xl"
		}
	}
	if numGPUs == 0 {
		numGPUs = 1
	}
	if vcpus == 0 {
		vcpus = utils.IncludedVCPUs(gpuType, numGPUs)
	}
	if diskSizeGB == 0 {
		diskSizeGB = 100
	}

	// Override with hovered option for current step
	switch m.step {
	case stepMode:
		modes := []string{"prototyping", "production"}
		mode = modes[m.cursor]
		// Reset dependent defaults
		if mode == "prototyping" {
			gpuType = "a6000"
		} else {
			gpuType = "a100xl"
		}
		numGPUs = 1
		vcpus = utils.IncludedVCPUs(gpuType, numGPUs)
	case stepGPU:
		gpus := m.getGPUOptions()
		gpuType = gpus[m.cursor]
		if numGPUs == 0 {
			numGPUs = 1
		}
		vcpus = utils.IncludedVCPUs(gpuType, numGPUs)
	case stepCompute:
		if m.gpuCountPhase {
			gpuCounts := []int{1, 2}
			numGPUs = gpuCounts[m.cursor]
			vcpus = utils.IncludedVCPUs(gpuType, numGPUs)
		} else if mode == "prototyping" {
			vcpuOpts := utils.PrototypingVCPUOptions(m.config.GPUType, m.config.NumGPUs)
			vcpus = vcpuOpts[m.cursor]
		} else {
			gpuOpts := []int{1, 2, 4, 8}
			numGPUs = gpuOpts[m.cursor]
			vcpus = 18 * numGPUs
		}
	case stepDiskSize:
		if v, err := fmt.Sscanf(m.diskInput.Value(), "%d", &diskSizeGB); v != 1 || err != nil {
			diskSizeGB = 100
		}
	}

	return utils.CalculateHourlyPrice(m.pricing, mode, gpuType, numGPUs, vcpus, diskSizeGB)
}

func RunCreateInteractive(client *api.Client) (*CreateConfig, error) {
	InitCommonStyles(os.Stdout)
	m := NewCreateModel(client)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result, ok := finalModel.(createModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if result.err != nil {
		return nil, result.err
	}

	if result.quitting || !result.config.Confirmed {
		return nil, ErrCancelled
	}

	return &result.config, nil
}
