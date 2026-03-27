package tui

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

const templatePageSize = 10

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

// CreatePresets holds flag values provided on the command line for hybrid mode.
// nil pointer means the flag was not set.
type CreatePresets struct {
	Mode       *string
	GPUType    *string
	NumGPUs    *int
	VCPUs      *int
	Template   *string
	DiskSizeGB *int
}

// IsEmpty returns true if no preset flags were set.
func (p *CreatePresets) IsEmpty() bool {
	return p.Mode == nil && p.GPUType == nil && p.NumGPUs == nil &&
		p.VCPUs == nil && p.Template == nil && p.DiskSizeGB == nil
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
	diskInputTouched bool
	err              error
	validationErr    error
	quitting         bool
	client           *api.Client
	spinner          spinner.Model
	selectedSnapshot *api.Snapshot
	gpuCountPhase    bool // when true, stepCompute shows GPU count selection before vCPU selection
	templateBrowse   bool // when true, stepTemplate shows full template/snapshot list
	templatePage     int  // current page when browsing templates
	pricing          *utils.PricingData
	pricingLoaded    bool
	specs            *utils.SpecStore
	presets          *CreatePresets
	skippedSteps     map[createStep]bool // records which steps were auto-filled
	styles           PanelStyles
}

func NewCreateModel(client *api.Client, specs *utils.SpecStore) createModel {
	styles := NewPanelStyles()
	s := NewPrimarySpinner()

	ti := textinput.New()
	ti.Placeholder = "100"
	ti.SetValue("100")
	ti.CharLimit = 4
	ti.Width = 20
	ti.Prompt = "▶ "

	return createModel{
		step:         stepMode,
		client:       client,
		spinner:      s,
		styles:       styles,
		specs:        specs,
		skippedSteps: make(map[createStep]bool),
		diskInput:    ti,
		config: CreateConfig{
			DiskSizeGB: 100,
		},
	}
}

// NewCreateModelWithPresets creates a createModel with pre-filled values from CLI flags.
func NewCreateModelWithPresets(client *api.Client, specs *utils.SpecStore, presets *CreatePresets) createModel {
	m := NewCreateModel(client, specs)
	m.presets = presets
	m.trySkipCurrentStep()
	return m
}

// resolveGPUForMode normalizes a user-provided GPU string and validates it
// against the given mode. Returns the canonical GPU type and true if valid.
func resolveGPUForMode(raw, mode string) (string, bool) {
	raw = strings.ToLower(raw)
	gpuMap := map[string]string{"a6000": "a6000", "a100": "a100xl", "h100": "h100"}
	canonical, ok := gpuMap[raw]
	if !ok {
		return "", false
	}
	if mode == "production" && canonical == "a6000" {
		return "", false
	}
	return canonical, true
}

// trySkipCurrentStep is the core hybrid-mode method. It loops forward through
// steps, auto-filling each one from presets if the preset value is valid given
// the current config state. Called after every step transition.
func (m *createModel) trySkipCurrentStep() tea.Cmd {
	for {
		skipped := false

		switch m.step {
		case stepMode:
			if m.presets != nil && m.presets.Mode != nil {
				mode := strings.ToLower(*m.presets.Mode)
				if mode == "prototyping" || mode == "production" {
					m.config.Mode = mode
					m.skippedSteps[stepMode] = true
					skipped = true
				}
			}

		case stepGPU:
			if m.presets != nil && m.presets.GPUType != nil {
				canonical, ok := resolveGPUForMode(*m.presets.GPUType, m.config.Mode)
				if ok {
					m.config.GPUType = canonical
					m.skippedSteps[stepGPU] = true
					skipped = true
				}
			}

		case stepCompute:
			skipped = m.trySkipCompute()

		case stepTemplate:
			return m.trySkipTemplate()

		case stepDiskSize:
			if m.presets != nil && m.presets.DiskSizeGB != nil {
				v := *m.presets.DiskSizeGB
				minDisk, maxDisk := m.specs.StorageRange(m.config.GPUType, m.config.NumGPUs, m.config.Mode)
				if v >= minDisk && v <= maxDisk {
					m.config.DiskSizeGB = v
					m.skippedSteps[stepDiskSize] = true
					skipped = true
				}
			}

		case stepConfirmation:
			m.initStep()
			return nil
		}

		if !skipped {
			m.initStep()
			return nil
		}

		m.step++
	}
}

// trySkipCompute handles the complex compute step with its sub-phases.
func (m *createModel) trySkipCompute() bool {
	if m.presets == nil {
		return false
	}

	gpuType := m.config.GPUType
	mode := m.config.Mode
	needsCount := m.specs.NeedsGPUCountPhase(gpuType, mode)

	if !needsCount {
		// Single-GPU type: numGPUs is always 1
		m.config.NumGPUs = 1
		if m.presets.VCPUs == nil {
			return false
		}
		if slices.Contains(m.specs.VCPUOptions(gpuType, 1, mode), *m.presets.VCPUs) {
			m.config.VCPUs = *m.presets.VCPUs
			return true
		}
		return false
	}

	// Multi-GPU type: need both num-gpus and vcpus to fully skip
	if m.presets.NumGPUs != nil && m.presets.VCPUs != nil {
		if slices.Contains(m.specs.GPUCountsForMode(gpuType, mode), *m.presets.NumGPUs) {
			vcpuOpts := m.specs.VCPUOptions(gpuType, *m.presets.NumGPUs, mode)
			if len(vcpuOpts) == 1 {
				// Single vCPU option (e.g. production) — auto-select
				m.config.NumGPUs = *m.presets.NumGPUs
				m.config.VCPUs = vcpuOpts[0]
				return true
			}
			if slices.Contains(vcpuOpts, *m.presets.VCPUs) {
				m.config.NumGPUs = *m.presets.NumGPUs
				m.config.VCPUs = *m.presets.VCPUs
				return true
			}
		}
		return false
	}

	// Only num-gpus provided
	if m.presets.NumGPUs != nil {
		if slices.Contains(m.specs.GPUCountsForMode(gpuType, mode), *m.presets.NumGPUs) {
			m.config.NumGPUs = *m.presets.NumGPUs
			// If single vCPU option, auto-select it too
			vcpuOpts := m.specs.VCPUOptions(gpuType, *m.presets.NumGPUs, mode)
			if len(vcpuOpts) == 1 {
				m.config.VCPUs = vcpuOpts[0]
				return true
			}
			m.gpuCountPhase = false
			return false // don't skip the whole step, just the sub-phase
		}
	}

	return false
}

// trySkipTemplate handles the template step, which depends on async-loaded data.
func (m *createModel) trySkipTemplate() tea.Cmd {
	if m.presets == nil || m.presets.Template == nil {
		m.initStep()
		return nil
	}

	if !m.templatesLoaded || !m.snapshotsLoaded {
		// Data not loaded yet — show spinner, re-attempt when data arrives
		return nil
	}

	raw := *m.presets.Template

	// Check templates by key or display name
	for _, t := range m.templates {
		if t.Key == raw || strings.EqualFold(t.Template.DisplayName, raw) {
			m.config.Template = t.Key
			m.selectedSnapshot = nil
			m.skippedSteps[stepTemplate] = true
			if m.presets.DiskSizeGB == nil {
				m.config.DiskSizeGB = 100
			}
			m.step++
			return m.trySkipCurrentStep() // continue the skip chain
		}
	}

	// Check snapshots by name
	for i, s := range m.snapshots {
		if s.Name == raw {
			m.config.Template = s.Name
			m.selectedSnapshot = &m.snapshots[i]
			m.skippedSteps[stepTemplate] = true
			if m.presets.DiskSizeGB == nil {
				m.config.DiskSizeGB = s.MinimumDiskSizeGB
			}
			m.step++
			return m.trySkipCurrentStep()
		}
	}

	// Not found — user picks manually
	m.initStep()
	return nil
}

// initStep sets up step-specific state (cursor, focus, sub-phases) when arriving at a visible step.
func (m *createModel) initStep() {
	m.cursor = 0
	switch m.step {
	case stepCompute:
		if m.specs.NeedsGPUCountPhase(m.config.GPUType, m.config.Mode) && m.config.NumGPUs == 0 {
			m.gpuCountPhase = true
		} else if !m.specs.NeedsGPUCountPhase(m.config.GPUType, m.config.Mode) {
			m.config.NumGPUs = 1
			m.gpuCountPhase = false
		}
	case stepDiskSize:
		m.diskInput.SetValue(fmt.Sprintf("%d", m.config.DiskSizeGB))
		m.diskInput.Focus()
		m.diskInputTouched = false
	}
}

// prevVisibleStep returns the previous non-skipped step. Returns -1 if none.
func (m *createModel) prevVisibleStep(from createStep) createStep {
	for s := from - 1; s >= stepMode; s-- {
		if !m.skippedSteps[s] {
			return s
		}
	}
	return -1
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
	rest := make([]api.TemplateEntry, 0, len(templates))

	for _, t := range templates {
		if strings.EqualFold(t.Key, "base") {
			sorted = append(sorted, t)
			break
		} else {
			rest = append(rest, t)
		}
	}

	sort.Slice(rest, func(i, j int) bool {
		return strings.ToLower(rest[i].Key) < strings.ToLower(rest[j].Key)
	})

	sorted = append(sorted, rest...)
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
		// If waiting on template step with a preset, try to skip now
		if m.step == stepTemplate && m.presets != nil && m.presets.Template != nil {
			return m, m.trySkipCurrentStep()
		}
		return m, m.spinner.Tick

	case createSnapshotsMsg:
		// Snapshots are optional, so ignore errors
		if msg.err == nil {
			m.snapshots = msg.snapshots
		}
		m.snapshotsLoaded = true
		// If waiting on template step with a preset, try to skip now
		if m.step == stepTemplate && m.presets != nil && m.presets.Template != nil {
			return m, m.trySkipCurrentStep()
		}
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
		// Forward key messages to disk input when on disk size step
		if m.step == stepDiskSize {
			switch msg.String() {
			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				prev := m.prevVisibleStep(m.step)
				if prev < 0 {
					m.quitting = true
					return m, tea.Quit
				}
				m.step = prev
				m.gpuCountPhase = false
				m.templateBrowse = false
				m.diskInput.Blur()
				m.initStep()
			case "enter":
				return m.handleEnter()
			default:
				if !m.diskInputTouched {
					m.diskInput.SetValue("")
					m.diskInputTouched = true
				}
				var cmd tea.Cmd
				m.diskInput, cmd = m.diskInput.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.step == stepTemplate && m.templateBrowse {
				// Go back to None/Browse phase
				m.templateBrowse = false
				m.templatePage = 0
				m.cursor = 0
			} else if m.step == stepCompute && !m.gpuCountPhase && m.specs.NeedsGPUCountPhase(m.config.GPUType, m.config.Mode) {
				// Go back to GPU count selection phase
				m.gpuCountPhase = true
				m.cursor = 0
			} else {
				prev := m.prevVisibleStep(m.step)
				if prev < 0 {
					m.quitting = true
					return m, tea.Quit
				}
				m.step = prev
				m.gpuCountPhase = false
				m.templateBrowse = false
				m.initStep()
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

		case "left", "h":
			if m.step == stepTemplate && m.templateBrowse && m.templatePage > 0 {
				m.templatePage--
				m.cursor = 0
			}

		case "right", "l":
			if m.step == stepTemplate && m.templateBrowse {
				totalItems := len(m.templates) + len(m.snapshots)
				maxPage := (totalItems - 1) / templatePageSize
				if m.templatePage < maxPage {
					m.templatePage++
					m.cursor = 0
				}
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
		return m, m.trySkipCurrentStep()

	case stepGPU:
		gpus := m.getGPUOptions()
		m.config.GPUType = gpus[m.cursor]
		m.step = stepCompute
		return m, m.trySkipCurrentStep()

	case stepCompute:
		if m.gpuCountPhase {
			gpuCounts := m.specs.GPUCountsForMode(m.config.GPUType, m.config.Mode)
			m.config.NumGPUs = gpuCounts[m.cursor]
			m.gpuCountPhase = false
			m.cursor = 0
			// Check if vCPUs preset can now be applied
			if m.presets != nil && m.presets.VCPUs != nil {
				if slices.Contains(m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs, m.config.Mode), *m.presets.VCPUs) {
					m.config.VCPUs = *m.presets.VCPUs
					m.step = stepTemplate
					return m, m.trySkipCurrentStep()
				}
			}
			// Stay on stepCompute to show vCPU options next
		} else {
			vcpuOpts := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs, m.config.Mode)
			if len(vcpuOpts) == 1 {
				// Single option (e.g. production) — auto-select
				m.config.VCPUs = vcpuOpts[0]
			} else {
				m.config.VCPUs = vcpuOpts[m.cursor]
			}
			m.step = stepTemplate
			return m, m.trySkipCurrentStep()
		}

	case stepTemplate:
		if !m.templateBrowse {
			// Phase 1: None / Browse selection
			if m.cursor == 0 {
				// "None" — use base template, advance to disk size
				m.config.Template = "base"
				m.selectedSnapshot = nil
				if m.presets == nil || m.presets.DiskSizeGB == nil {
					m.config.DiskSizeGB = 100
				}
				m.step = stepDiskSize
				return m, m.trySkipCurrentStep()
			}
			// "Browse Templates..." — enter browse phase
			m.templateBrowse = true
			m.templatePage = 0
			m.cursor = 0
		} else {
			// Phase 2: Full template/snapshot selection (paginated)
			absIndex := m.templatePage*templatePageSize + m.cursor
			totalOptions := len(m.templates) + len(m.snapshots)
			if absIndex < totalOptions {
				if absIndex < len(m.templates) {
					m.config.Template = m.templates[absIndex].Key
					m.selectedSnapshot = nil
					if m.presets == nil || m.presets.DiskSizeGB == nil {
						m.config.DiskSizeGB = 100
					}
				} else {
					snapshotIndex := absIndex - len(m.templates)
					snapshot := m.snapshots[snapshotIndex]
					m.config.Template = snapshot.Name
					m.selectedSnapshot = &snapshot
					if m.presets == nil || m.presets.DiskSizeGB == nil {
						m.config.DiskSizeGB = snapshot.MinimumDiskSizeGB
					}
				}
				m.step = stepDiskSize
				return m, m.trySkipCurrentStep()
			}
		}

	case stepDiskSize:
		minDisk, maxDisk := m.specs.StorageRange(m.config.GPUType, m.config.NumGPUs, m.config.Mode)
		if m.selectedSnapshot != nil && m.selectedSnapshot.MinimumDiskSizeGB > minDisk {
			minDisk = m.selectedSnapshot.MinimumDiskSizeGB
		}
		diskSize, err := strconv.Atoi(m.diskInput.Value())
		if err != nil || diskSize < minDisk || diskSize > maxDisk {
			m.validationErr = fmt.Errorf("disk size must be between %d and %d GB", minDisk, maxDisk)
			return m, nil
		}
		m.config.DiskSizeGB = diskSize
		m.validationErr = nil
		m.diskInput.Blur()
		m.step = stepConfirmation
		return m, m.trySkipCurrentStep()

	case stepConfirmation:
		if m.cursor == 0 {
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
	return m.specs.GPUOptionsForMode(m.config.Mode)
}

func (m createModel) getMaxCursor() int {
	switch m.step {
	case stepMode:
		return 1
	case stepGPU:
		return len(m.getGPUOptions()) - 1
	case stepCompute:
		if m.gpuCountPhase {
			return len(m.specs.GPUCountsForMode(m.config.GPUType, m.config.Mode)) - 1
		}
		return len(m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs, m.config.Mode)) - 1
	case stepTemplate:
		if !m.templateBrowse {
			return 1 // None / Browse
		}
		totalItems := len(m.templates) + len(m.snapshots)
		pageStart := m.templatePage * templatePageSize
		return min(totalItems-pageStart, templatePageSize) - 1
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
	for i, stepName := range progressSteps {
		adjustedStep := int(m.step)
		if i == adjustedStep {
			s.WriteString(m.styles.Selected.Render(fmt.Sprintf("[%s]", stepName)))
		} else if i < adjustedStep || m.skippedSteps[createStep(i)] {
			s.WriteString(fmt.Sprintf("[✓ %s]", stepName))
		} else {
			s.WriteString(fmt.Sprintf("[%s]", stepName))
		}
		if i < len(progressSteps)-1 {
			s.WriteString(" → ")
		}
	}
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
			gpuCounts := m.specs.GPUCountsForMode(m.config.GPUType, m.config.Mode)
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
		} else {
			ramPerVCPU := m.specs.RamPerVCPU(m.config.GPUType, m.config.NumGPUs, m.config.Mode)
			s.WriteString(fmt.Sprintf("Select vCPU count (%dGB RAM per vCPU):\n\n", ramPerVCPU))
			vcpuOpts := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs, m.config.Mode)
			for i, vcpu := range vcpuOpts {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.Cursor.Render("▶ ")
				}
				ram := vcpu * ramPerVCPU
				line := fmt.Sprintf("%s%d vCPUs (%d GB RAM)", cursor, vcpu, ram)
				if m.cursor == i {
					line = fmt.Sprintf("%s%s", cursor, m.styles.Selected.Render(fmt.Sprintf("%d vCPUs (%d GB RAM)", vcpu, ram)))
				}
				s.WriteString(line + "\n")
			}
		}

	case stepTemplate:
		if !m.templateBrowse {
			// Phase 1: None / Browse
			s.WriteString("Select environment template:\n\n")
			options := []string{"None (Base ML Environment)", "Browse Templates..."}
			for i, opt := range options {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.Cursor.Render("▶ ")
				}
				display := opt
				if m.cursor == i {
					display = m.styles.Selected.Render(opt)
				}
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, display))
			}
		} else {
			// Phase 2: Paginated template/snapshot list
			if !m.templatesLoaded || !m.snapshotsLoaded {
				s.WriteString("Select a template:\n\n")
				s.WriteString(fmt.Sprintf("%s Loading options...\n", m.spinner.View()))
			} else {
				totalItems := len(m.templates) + len(m.snapshots)
				totalPages := (totalItems + templatePageSize - 1) / templatePageSize
				pageStart := m.templatePage * templatePageSize
				pageEnd := min(pageStart+templatePageSize, totalItems)

				s.WriteString(fmt.Sprintf("Select a template (page %d/%d):\n\n", m.templatePage+1, totalPages))

				// Track local cursor index within the page
				localIdx := 0
				// Show section labels when the section starts on or spans this page
				if pageStart < len(m.templates) {
					s.WriteString(m.styles.Label.Render("Templates:") + "\n")
				}
				for i := pageStart; i < pageEnd && i < len(m.templates); i++ {
					entry := m.templates[i]
					cursor := "  "
					if m.cursor == localIdx {
						cursor = m.styles.Cursor.Render("▶ ")
					}
					name := entry.Template.DisplayName
					if entry.Template.ExtendedDescription != "" {
						name += fmt.Sprintf(" - %s", entry.Template.ExtendedDescription)
					}
					if m.cursor == localIdx {
						name = m.styles.Selected.Render(name)
					}
					s.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
					localIdx++
				}

				if pageEnd > len(m.templates) && len(m.snapshots) > 0 {
					snapshotStart := 0
					if pageStart > len(m.templates) {
						snapshotStart = pageStart - len(m.templates)
					}
					snapshotEnd := min(pageEnd-len(m.templates), len(m.snapshots))

					s.WriteString("\n")
					s.WriteString(m.styles.Label.Render("Custom Snapshots:") + "\n")
					for i := snapshotStart; i < snapshotEnd; i++ {
						snapshot := m.snapshots[i]
						cursor := "  "
						if m.cursor == localIdx {
							cursor = m.styles.Cursor.Render("▶ ")
						}
						name := fmt.Sprintf("%s (%d GB)", snapshot.Name, snapshot.MinimumDiskSizeGB)
						if m.cursor == localIdx {
							name = m.styles.Selected.Render(name)
						}
						s.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
						localIdx++
					}
				}
			}
		}

	case stepDiskSize:
		minDisk, maxDisk := m.specs.StorageRange(m.config.GPUType, m.config.NumGPUs, m.config.Mode)
		if m.selectedSnapshot != nil && m.selectedSnapshot.MinimumDiskSizeGB > minDisk {
			minDisk = m.selectedSnapshot.MinimumDiskSizeGB
		}
		s.WriteString("Enter disk size (GB):\n\n")
		s.WriteString(fmt.Sprintf("Range: %d-%d GB\n\n", minDisk, maxDisk))
		s.WriteString(m.diskInput.View())
		s.WriteString("\n")
		if m.validationErr != nil {
			s.WriteString(fmt.Sprintf("\n%s\n", errorStyleTUI.Render(fmt.Sprintf("✗ %v", m.validationErr))))
		}

	case stepConfirmation:
		s.WriteString("Review your configuration:\n")

		var panel strings.Builder
		panel.WriteString(m.styles.Label.Render("Mode:       ") + utils.Capitalize(m.config.Mode) + "\n")
		panel.WriteString(m.styles.Label.Render("Template:   ") + utils.Capitalize(m.config.Template) + "\n")
		panel.WriteString(m.styles.Label.Render("GPU Type:   ") + utils.FormatGPUType(m.config.GPUType) + "\n")
		panel.WriteString(m.styles.Label.Render("GPUs:       ") + strconv.Itoa(m.config.NumGPUs) + "\n")
		panel.WriteString(m.styles.Label.Render("vCPUs:      ") + strconv.Itoa(m.config.VCPUs) + "\n")
		confirmRamPerVCPU := m.specs.RamPerVCPU(m.config.GPUType, m.config.NumGPUs, m.config.Mode)
		panel.WriteString(m.styles.Label.Render("RAM:        ") + strconv.Itoa(m.config.VCPUs*confirmRamPerVCPU) + " GB\n")
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
		if m.step == stepTemplate && m.templateBrowse {
			totalItems := len(m.templates) + len(m.snapshots)
			if totalItems > templatePageSize {
				s.WriteString(m.styles.Help.Render("↑/↓: Navigate  ←/→: Page  Enter: Select  Esc: Back  Q: Cancel\n"))
			} else {
				s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Cancel\n"))
			}
		} else {
			s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Cancel\n"))
		}
	} else {
		s.WriteString("\n")
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Confirm  Esc: Back  Q: Cancel\n"))
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
		gpuOpts := m.specs.GPUOptionsForMode(mode)
		if len(gpuOpts) > 0 {
			gpuType = gpuOpts[0]
		}
	}
	if numGPUs == 0 {
		numGPUs = 1
	}
	if vcpus == 0 {
		vcpus = m.specs.IncludedVCPUs(gpuType, numGPUs, mode)
	}
	if diskSizeGB == 0 {
		diskSizeGB = 100
	}

	// Override with hovered option for current step
	switch m.step {
	case stepMode:
		modes := []string{"prototyping", "production"}
		mode = modes[m.cursor]
		gpuOpts := m.specs.GPUOptionsForMode(mode)
		if len(gpuOpts) > 0 {
			gpuType = gpuOpts[0]
		}
		numGPUs = 1
		vcpus = m.specs.IncludedVCPUs(gpuType, numGPUs, mode)
	case stepGPU:
		gpus := m.getGPUOptions()
		gpuType = gpus[m.cursor]
		if numGPUs == 0 {
			numGPUs = 1
		}
		vcpus = m.specs.IncludedVCPUs(gpuType, numGPUs, mode)
	case stepCompute:
		if m.gpuCountPhase {
			gpuCounts := m.specs.GPUCountsForMode(gpuType, mode)
			numGPUs = gpuCounts[m.cursor]
			vcpus = m.specs.IncludedVCPUs(gpuType, numGPUs, mode)
		} else {
			vcpuOpts := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs, mode)
			vcpus = vcpuOpts[m.cursor]
		}
	case stepDiskSize:
		if v, err := strconv.Atoi(m.diskInput.Value()); err == nil && v >= 10 {
			diskSizeGB = v
		}
	}

	included := m.specs.IncludedVCPUs(gpuType, numGPUs, mode)
	return utils.CalculateHourlyPrice(m.pricing, mode, gpuType, numGPUs, vcpus, diskSizeGB, included)
}

func runCreateModel(m createModel) (*CreateConfig, error) {
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

func RunCreateInteractive(client *api.Client, specs *utils.SpecStore) (*CreateConfig, error) {
	InitCommonStyles(os.Stdout)
	m := NewCreateModel(client, specs)
	return runCreateModel(m)
}

// RunCreateHybrid runs the create TUI with some steps pre-filled from CLI flags.
func RunCreateHybrid(client *api.Client, specs *utils.SpecStore, presets *CreatePresets) (*CreateConfig, error) {
	InitCommonStyles(os.Stdout)
	m := NewCreateModelWithPresets(client, specs, presets)
	return runCreateModel(m)
}
