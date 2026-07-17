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

const (
	templateWindowSize   = 10
	includedDiskGBPerGPU = 100
)

type createStep int

const (
	stepGPU createStep = iota
	stepCompute
	stepTemplate
	stepDiskSize
	stepConfirmation
	stepComplete
)

// CreateConfig holds the configuration for creating an instance
type CreateConfig struct {
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
	GPUType    *string
	NumGPUs    *int
	VCPUs      *int
	Template   *string
	DiskSizeGB *int
}

// IsEmpty returns true if no preset flags were set.
func (p *CreatePresets) IsEmpty() bool {
	return p.GPUType == nil && p.NumGPUs == nil && p.VCPUs == nil &&
		p.Template == nil && p.DiskSizeGB == nil
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
	templateBrowse   bool // when true, stepTemplate shows full template list
	templateOffset   int  // index of first visible item when browsing templates
	snapshotBrowse   bool // when true, stepTemplate shows snapshot list
	snapshotOffset   int  // index of first visible item when browsing snapshots
	pricing          *utils.PricingData
	pricingLoaded    bool
	pricingErr       error
	specs            *utils.SpecStore
	specsLoaded      bool
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

	m := createModel{
		step:         stepGPU,
		client:       client,
		spinner:      s,
		styles:       styles,
		skippedSteps: make(map[createStep]bool),
		diskInput:    ti,
		config: CreateConfig{
			DiskSizeGB: 100,
		},
	}
	if specs != nil {
		m.specs = specs
		m.specsLoaded = true
	}
	return m
}

// NewCreateModelWithPresets creates a createModel with pre-filled values from CLI flags.
func NewCreateModelWithPresets(client *api.Client, specs *utils.SpecStore, presets *CreatePresets) createModel {
	m := NewCreateModel(client, specs)
	m.presets = presets
	m.trySkipCurrentStep()
	return m
}

func defaultCreateDiskSizeForGPUCount(numGPUs int) int {
	if numGPUs <= 0 {
		numGPUs = 1
	}
	return numGPUs * includedDiskGBPerGPU
}

func (m createModel) defaultDiskSizeGB() int {
	diskSizeGB := defaultCreateDiskSizeForGPUCount(m.config.NumGPUs)
	if m.selectedSnapshot != nil && m.selectedSnapshot.MinimumDiskSizeGB > diskSizeGB {
		diskSizeGB = m.selectedSnapshot.MinimumDiskSizeGB
	}
	if m.specs == nil || m.config.GPUType == "" {
		return diskSizeGB
	}

	minDisk, maxDisk := m.specs.StorageRange(m.config.GPUType, m.config.NumGPUs)
	if m.selectedSnapshot != nil && m.selectedSnapshot.MinimumDiskSizeGB > maxDisk {
		maxDisk = m.selectedSnapshot.MinimumDiskSizeGB
	}
	if diskSizeGB < minDisk {
		return minDisk
	}
	if diskSizeGB > maxDisk {
		return maxDisk
	}
	return diskSizeGB
}

func (m *createModel) setDefaultDiskSize() {
	if m.presets != nil && m.presets.DiskSizeGB != nil {
		return
	}
	m.config.DiskSizeGB = m.defaultDiskSizeGB()
}

// trySkipCurrentStep is the core hybrid-mode method. It loops forward through
// steps, auto-filling each one from presets if the preset value is valid given
// the current config state. Called after every step transition.
func (m *createModel) trySkipCurrentStep() tea.Cmd {
	for {
		skipped := false

		switch m.step {
		case stepGPU:
			if m.presets != nil && m.presets.GPUType != nil && m.specs != nil {
				canonical, ok := m.specs.NormalizeGPUType(*m.presets.GPUType)
				if ok && m.specs.IsGPUTypeAvailable(canonical) {
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
				minDisk, maxDisk := m.specs.StorageRange(m.config.GPUType, m.config.NumGPUs)
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
	needsCount := m.specs.NeedsGPUCountPhase(gpuType)

	if !needsCount {
		// Single-GPU type: numGPUs is always 1
		m.config.NumGPUs = 1
		m.setDefaultDiskSize()
		if !m.specs.IsSpecAvailable(gpuType, 1) {
			return false
		}
		if m.presets.VCPUs == nil {
			return false
		}
		if slices.Contains(m.specs.VCPUOptions(gpuType, 1), *m.presets.VCPUs) {
			m.config.VCPUs = *m.presets.VCPUs
			return true
		}
		return false
	}

	// Multi-GPU type: need both num-gpus and vcpus to fully skip
	if m.presets.NumGPUs != nil && m.presets.VCPUs != nil {
		if slices.Contains(m.specs.GPUCounts(gpuType), *m.presets.NumGPUs) && m.specs.IsSpecAvailable(gpuType, *m.presets.NumGPUs) {
			vcpuOpts := m.specs.VCPUOptions(gpuType, *m.presets.NumGPUs)
			if len(vcpuOpts) == 1 {
				// Single vCPU option: auto-select it.
				m.config.NumGPUs = *m.presets.NumGPUs
				m.setDefaultDiskSize()
				m.config.VCPUs = vcpuOpts[0]
				return true
			}
			if slices.Contains(vcpuOpts, *m.presets.VCPUs) {
				m.config.NumGPUs = *m.presets.NumGPUs
				m.setDefaultDiskSize()
				m.config.VCPUs = *m.presets.VCPUs
				return true
			}
		}
		return false
	}

	// Only num-gpus provided
	if m.presets.NumGPUs != nil {
		if slices.Contains(m.specs.GPUCounts(gpuType), *m.presets.NumGPUs) && m.specs.IsSpecAvailable(gpuType, *m.presets.NumGPUs) {
			m.config.NumGPUs = *m.presets.NumGPUs
			m.setDefaultDiskSize()
			// If single vCPU option, auto-select it too
			vcpuOpts := m.specs.VCPUOptions(gpuType, *m.presets.NumGPUs)
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
				m.config.DiskSizeGB = m.defaultDiskSizeGB()
			}
			m.step++
			return m.trySkipCurrentStep() // continue the skip chain
		}
	}

	// Check snapshots by name
	for i, s := range m.snapshots {
		if s.Name == raw {
			if !s.IsReady() {
				m.openSnapshotBrowserAt(i)
				return nil
			}
			m.config.Template = s.Name
			m.selectedSnapshot = &m.snapshots[i]
			m.skippedSteps[stepTemplate] = true
			if m.presets.DiskSizeGB == nil {
				m.config.DiskSizeGB = m.defaultDiskSizeGB()
			}
			m.step++
			return m.trySkipCurrentStep()
		}
	}

	// Not found — user picks manually
	m.initStep()
	return nil
}

func (m *createModel) openSnapshotBrowserAt(index int) {
	if index < 0 || index >= len(m.snapshots) {
		index = 0
	}
	m.templateBrowse = false
	m.snapshotBrowse = true
	m.cursor = index
	m.snapshotOffset = 0
	if index >= templateWindowSize {
		m.snapshotOffset = index - templateWindowSize + 1
	}
}

// initStep sets up step-specific state (cursor, focus, sub-phases) when arriving at a visible step.
func (m *createModel) initStep() {
	m.cursor = 0
	switch m.step {
	case stepGPU:
		gpus := m.getGPUOptions()
		for i, gpu := range gpus {
			if m.specs.IsGPUTypeAvailable(gpu) {
				m.cursor = i
				break
			}
		}
	case stepCompute:
		if m.specs.NeedsGPUCountPhase(m.config.GPUType) && m.config.NumGPUs == 0 {
			m.gpuCountPhase = true
			for i, count := range m.specs.GPUCounts(m.config.GPUType) {
				if m.specs.IsSpecAvailable(m.config.GPUType, count) {
					m.cursor = i
					break
				}
			}
		} else if !m.specs.NeedsGPUCountPhase(m.config.GPUType) {
			m.config.NumGPUs = 1
			m.setDefaultDiskSize()
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
	for s := from - 1; s >= stepGPU; s-- {
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

type createSpecsMsg struct {
	specs *utils.SpecStore
	err   error
}

func snapshotStatusLabel(snapshot api.Snapshot) string {
	status := snapshot.NormalizedStatus()
	if snapshot.IsReady() {
		return status
	}
	if status == "CREATING" {
		return status + " (not ready)"
	}
	return status + " (unavailable)"
}

func sortTemplates(templates []api.TemplateEntry) []api.TemplateEntry {
	var base *api.TemplateEntry
	rest := make([]api.TemplateEntry, 0, len(templates))

	for i, t := range templates {
		if strings.EqualFold(t.Key, "base") {
			base = &templates[i]
		} else {
			rest = append(rest, t)
		}
	}

	sort.Slice(rest, func(i, j int) bool {
		return strings.ToLower(rest[i].Template.DisplayName) < strings.ToLower(rest[j].Template.DisplayName)
	})

	if base != nil {
		return append([]api.TemplateEntry{*base}, rest...)
	}
	return rest
}

func fetchCreateTemplatesCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		templates, err := client.ListTemplates()
		return createTemplatesMsg{templates: templates, err: err}
	}
}

func fetchCreateSnapshotsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		snapshots, err := client.ListSnapshots()
		return createSnapshotsMsg{snapshots: []api.Snapshot(snapshots), err: err}
	}
}

func fetchCreatePricingCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		rates, err := client.FetchPricing()
		return createPricingMsg{rates: rates, err: err}
	}
}

func fetchCreateSpecsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		specs, err := utils.FetchSpecStore(client)
		if err != nil {
			return createSpecsMsg{err: err}
		}
		return createSpecsMsg{specs: specs}
	}
}

func (m createModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		fetchCreateTemplatesCmd(m.client),
		fetchCreateSnapshotsCmd(m.client),
		fetchCreatePricingCmd(m.client),
		m.spinner.Tick,
	}
	if !m.specsLoaded {
		cmds = append(cmds, fetchCreateSpecsCmd(m.client))
	}
	return tea.Batch(cmds...)
}

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case createTemplatesMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.templates = sortTemplates(msg.templates)
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
		} else {
			m.pricingErr = msg.err
		}
		m.pricingLoaded = true
		return m, nil

	case createSpecsMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.specs = msg.specs
		m.specsLoaded = true
		// If waiting on a step that needs specs with presets, try to skip
		if m.presets != nil {
			return m, m.trySkipCurrentStep()
		}
		// Flagless flow: the user may have already advanced to a specs-dependent
		// step while specs were loading. Initialize its cursor now that options
		// are known (otherwise it stays at the default 0).
		if m.step == stepGPU || m.step == stepCompute {
			m.initStep()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		// Keep spinning if async data hasn't loaded yet (including pricing, which
		// drives the "calculating…" indicator on the estimated-cost line).
		if !m.templatesLoaded || !m.snapshotsLoaded || !m.specsLoaded || !m.pricingLoaded {
			return m, tea.Batch(cmd, m.spinner.Tick)
		}
		return m, cmd

	case tea.KeyMsg:
		// Forward key messages to text inputs on disk steps
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
		case "q", "Q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.step == stepTemplate && (m.templateBrowse || m.snapshotBrowse) {
				// Go back to None/Browse/Snapshots phase
				m.templateBrowse = false
				m.snapshotBrowse = false
				m.templateOffset = 0
				m.snapshotOffset = 0
				m.cursor = 0
			} else if m.step == stepCompute && !m.gpuCountPhase && m.specs.NeedsGPUCountPhase(m.config.GPUType) {
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
				m.snapshotBrowse = false
				m.initStep()
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.step == stepTemplate && m.templateBrowse && m.cursor < m.templateOffset {
					m.templateOffset = m.cursor
				}
				if m.step == stepTemplate && m.snapshotBrowse && m.cursor < m.snapshotOffset {
					m.snapshotOffset = m.cursor
				}
			}

		case "down", "j":
			maxCursor := m.getMaxCursor()
			if m.cursor < maxCursor {
				m.cursor++
				if m.step == stepTemplate && m.templateBrowse && m.cursor >= m.templateOffset+templateWindowSize {
					m.templateOffset = m.cursor - templateWindowSize + 1
				}
				if m.step == stepTemplate && m.snapshotBrowse && m.cursor >= m.snapshotOffset+templateWindowSize {
					m.snapshotOffset = m.cursor - templateWindowSize + 1
				}
			}

		}
	}

	return m, nil
}

func (m createModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepGPU:
		if !m.specsLoaded {
			return m, nil
		}
		gpus := m.getGPUOptions()
		if !m.specs.IsGPUTypeAvailable(gpus[m.cursor]) {
			return m, nil
		}
		m.config.GPUType = gpus[m.cursor]
		m.step = stepCompute
		return m, m.trySkipCurrentStep()

	case stepCompute:
		if m.gpuCountPhase {
			gpuCounts := m.specs.GPUCounts(m.config.GPUType)
			if !m.specs.IsSpecAvailable(m.config.GPUType, gpuCounts[m.cursor]) {
				return m, nil
			}
			m.config.NumGPUs = gpuCounts[m.cursor]
			m.setDefaultDiskSize()
			m.gpuCountPhase = false
			m.cursor = 0
			// Check if vCPUs preset can now be applied
			if m.presets != nil && m.presets.VCPUs != nil {
				if slices.Contains(m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs), *m.presets.VCPUs) {
					m.config.VCPUs = *m.presets.VCPUs
					m.step = stepTemplate
					return m, m.trySkipCurrentStep()
				}
			}
			// Stay on stepCompute to show vCPU options next
		} else {
			vcpuOpts := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs)
			if len(vcpuOpts) == 1 {
				// Single option: auto-select.
				m.config.VCPUs = vcpuOpts[0]
			} else {
				m.config.VCPUs = vcpuOpts[m.cursor]
			}
			m.step = stepTemplate
			return m, m.trySkipCurrentStep()
		}

	case stepTemplate:
		if m.templateBrowse {
			// Scrolling template list
			if m.cursor < len(m.templates) {
				m.config.Template = m.templates[m.cursor].Key
				m.selectedSnapshot = nil
				if m.presets == nil || m.presets.DiskSizeGB == nil {
					m.config.DiskSizeGB = m.defaultDiskSizeGB()
				}
				m.step = stepDiskSize
				return m, m.trySkipCurrentStep()
			}
		} else if m.snapshotBrowse {
			// Scrolling snapshot list
			if m.cursor < len(m.snapshots) {
				snapshot := m.snapshots[m.cursor]
				if !snapshot.IsReady() {
					return m, nil
				}
				m.config.Template = snapshot.Name
				m.selectedSnapshot = &snapshot
				if m.presets == nil || m.presets.DiskSizeGB == nil {
					m.config.DiskSizeGB = m.defaultDiskSizeGB()
				}
				m.step = stepDiskSize
				return m, m.trySkipCurrentStep()
			}
		} else {
			// Phase 1: None / Browse Templates / Custom Snapshots
			switch m.cursor {
			case 0:
				// "None" — use base template
				m.config.Template = "base"
				m.selectedSnapshot = nil
				if m.presets == nil || m.presets.DiskSizeGB == nil {
					m.config.DiskSizeGB = m.defaultDiskSizeGB()
				}
				m.step = stepDiskSize
				return m, m.trySkipCurrentStep()
			case 1:
				// "Browse Templates"
				m.templateBrowse = true
				m.templateOffset = 0
				m.cursor = 0
			case 2:
				// "Custom Snapshots"
				m.openSnapshotBrowserAt(0)
			}
		}

	case stepDiskSize:
		minDisk, maxDisk := m.specs.StorageRange(m.config.GPUType, m.config.NumGPUs)
		if m.selectedSnapshot != nil {
			if m.selectedSnapshot.MinimumDiskSizeGB > minDisk {
				minDisk = m.selectedSnapshot.MinimumDiskSizeGB
			}
			if m.selectedSnapshot.MinimumDiskSizeGB > maxDisk {
				maxDisk = m.selectedSnapshot.MinimumDiskSizeGB
			}
		}
		diskSize, err := strconv.Atoi(m.diskInput.Value())
		if err != nil || diskSize < minDisk || diskSize > maxDisk {
			m.validationErr = fmt.Errorf("primary storage for this instance type must be between %d and %d GB. Check storage limits at https://www.thundercompute.com/pricing", minDisk, maxDisk)
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
	if m.specs == nil {
		return nil
	}
	return m.specs.GPUOptions()
}

func (m createModel) getMaxCursor() int {
	switch m.step {
	case stepGPU:
		return len(m.getGPUOptions()) - 1
	case stepCompute:
		if m.gpuCountPhase {
			return len(m.specs.GPUCounts(m.config.GPUType)) - 1
		}
		return len(m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs)) - 1
	case stepTemplate:
		if m.templateBrowse {
			return len(m.templates) - 1
		}
		if m.snapshotBrowse {
			return len(m.snapshots) - 1
		}
		// None / Browse Templates / Custom Snapshots
		if m.snapshotsLoaded && len(m.snapshots) > 0 {
			return 2
		}
		return 1
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

	type progressEntry struct {
		name  string
		steps []createStep
	}
	progressSteps := []progressEntry{
		{"GPU", []createStep{stepGPU}},
		{"Size", []createStep{stepCompute}},
		{"Template", []createStep{stepTemplate}},
		{"Disk", []createStep{stepDiskSize}},
		{"Confirm", []createStep{stepConfirmation}},
	}
	for i, entry := range progressSteps {
		isCurrent := false
		isDone := true
		for _, st := range entry.steps {
			if st == m.step {
				isCurrent = true
			}
			if st <= m.step || m.skippedSteps[st] {
				// step is done
			} else {
				isDone = false
			}
		}
		if isCurrent {
			s.WriteString(m.styles.Selected.Render(fmt.Sprintf("[%s]", entry.name)))
		} else if isDone {
			s.WriteString(fmt.Sprintf("[✓ %s]", entry.name))
		} else {
			s.WriteString(fmt.Sprintf("[%s]", entry.name))
		}
		if i < len(progressSteps)-1 {
			s.WriteString(" → ")
		}
	}
	s.WriteString("\n\n")

	switch m.step {
	case stepGPU:
		if !m.specsLoaded {
			s.WriteString("Select GPU type:\n\n")
			s.WriteString(fmt.Sprintf("%s Loading GPU options...\n", m.spinner.View()))
			break
		}
		s.WriteString("Select GPU type:\n\n")
		gpus := m.getGPUOptions()
		for i, gpu := range gpus {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.Cursor.Render("▶ ")
			}
			displayName := utils.FormatGPUType(gpu)

			if !m.specs.IsGPUTypeAvailable(gpu) {
				displayName += " (unavailable)"
				displayName = subtleTextStyle.Render(displayName)
			} else if m.cursor == i {
				displayName = m.styles.Selected.Render(displayName)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, displayName))
		}
		if len(gpus) > 0 && !m.specs.IsGPUTypeAvailable(gpus[m.cursor]) {
			s.WriteString("\n")
			s.WriteString(warningStyleTUI.Render("This GPU type is currently unavailable. Choose another GPU type."))
			s.WriteString("\n")
		}

	case stepCompute:
		if m.gpuCountPhase {
			s.WriteString("Select number of GPUs:\n\n")
			gpuCounts := m.specs.GPUCounts(m.config.GPUType)
			for i, num := range gpuCounts {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.Cursor.Render("▶ ")
				}
				text := fmt.Sprintf("%d GPU(s)", num)
				if !m.specs.IsSpecAvailable(m.config.GPUType, num) {
					text = subtleTextStyle.Render(text + " (unavailable)")
				} else if m.cursor == i {
					text = m.styles.Selected.Render(text)
				}
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, text))
			}
			if len(gpuCounts) > 0 && !m.specs.IsSpecAvailable(m.config.GPUType, gpuCounts[m.cursor]) {
				s.WriteString("\n")
				s.WriteString(warningStyleTUI.Render("This GPU count is currently unavailable. Choose another count."))
				s.WriteString("\n")
			}
		} else {
			ramPerVCPU := m.specs.RamPerVCPU(m.config.GPUType, m.config.NumGPUs)
			s.WriteString(fmt.Sprintf("Select vCPU count (%dGB RAM per vCPU):\n\n", ramPerVCPU))
			vcpuOpts := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs)
			for i, vcpu := range vcpuOpts {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.Cursor.Render("▶ ")
				}
				ram := m.specs.RamForVCPU(m.config.GPUType, m.config.NumGPUs, vcpu)
				line := fmt.Sprintf("%s%d vCPUs (%d GB RAM)", cursor, vcpu, ram)
				if m.cursor == i {
					line = fmt.Sprintf("%s%s", cursor, m.styles.Selected.Render(fmt.Sprintf("%d vCPUs (%d GB RAM)", vcpu, ram)))
				}
				s.WriteString(line + "\n")
			}
		}

	case stepTemplate:
		if m.templateBrowse {
			// Scrolling template list
			if !m.templatesLoaded {
				s.WriteString("Select a template:\n\n")
				s.WriteString(fmt.Sprintf("%s Loading options...\n", m.spinner.View()))
			} else {
				totalItems := len(m.templates)
				winStart := m.templateOffset
				winEnd := min(winStart+templateWindowSize, totalItems)

				s.WriteString("Select a template:\n\n")

				if winStart > 0 {
					s.WriteString(m.styles.Help.Render(fmt.Sprintf("  ↑ %d more", winStart)) + "\n")
				} else {
					s.WriteString("\n")
				}

				for i := winStart; i < winEnd; i++ {
					entry := m.templates[i]
					cursor := "  "
					if m.cursor == i {
						cursor = m.styles.Cursor.Render("▶ ")
					}
					name := entry.Template.DisplayName
					if strings.EqualFold(entry.Key, "base") {
						name += " (Default)"
					}
					if entry.Template.ExtendedDescription != "" {
						name += fmt.Sprintf(" - %s", entry.Template.ExtendedDescription)
					}
					if m.cursor == i {
						name = m.styles.Selected.Render(name)
					}
					s.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
				}

				if winEnd < totalItems {
					s.WriteString(m.styles.Help.Render(fmt.Sprintf("  ↓ %d more", totalItems-winEnd)) + "\n")
				} else {
					s.WriteString("\n")
				}
			}
		} else if m.snapshotBrowse {
			// Scrolling snapshot list
			if !m.snapshotsLoaded {
				s.WriteString("Select a snapshot:\n\n")
				s.WriteString(fmt.Sprintf("%s Loading options...\n", m.spinner.View()))
			} else {
				totalItems := len(m.snapshots)
				winStart := m.snapshotOffset
				winEnd := min(winStart+templateWindowSize, totalItems)

				s.WriteString("Select a snapshot:\n\n")

				if winStart > 0 {
					s.WriteString(m.styles.Help.Render(fmt.Sprintf("  ↑ %d more", winStart)) + "\n")
				} else {
					s.WriteString("\n")
				}

				for i := winStart; i < winEnd; i++ {
					snapshot := m.snapshots[i]
					cursor := "  "
					if m.cursor == i {
						cursor = m.styles.Cursor.Render("▶ ")
					}
					name := fmt.Sprintf("%s (%d GB) — %s", snapshot.Name, snapshot.MinimumDiskSizeGB, snapshotStatusLabel(snapshot))
					if !snapshot.IsReady() {
						name = subtleTextStyle.Render(name)
					} else if m.cursor == i {
						name = m.styles.Selected.Render(name)
					}
					s.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
				}

				if winEnd < totalItems {
					s.WriteString(m.styles.Help.Render(fmt.Sprintf("  ↓ %d more", totalItems-winEnd)) + "\n")
				} else {
					s.WriteString("\n")
				}

				if m.cursor < len(m.snapshots) && !m.snapshots[m.cursor].IsReady() {
					snapshot := m.snapshots[m.cursor]
					s.WriteString(warningStyleTUI.Render(fmt.Sprintf(
						"Snapshot %q is %s. Only READY snapshots can be used for creation.",
						snapshot.Name,
						snapshot.NormalizedStatus(),
					)))
					s.WriteString("\n")
				}
			}
		} else {
			// Phase 1: None / Browse Templates / Custom Snapshots
			s.WriteString("Select environment template:\n\n")
			options := []string{"None (Base ML Environment)", "Browse Templates"}
			if m.snapshotsLoaded && len(m.snapshots) > 0 {
				options = append(options, "Custom Snapshots")
			}
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
		}

	case stepDiskSize:
		s.WriteString("Configure storage:\n\n")
		s.WriteString(m.styles.Selected.Render("▶ Primary") + "\n")
		s.WriteString("  " + m.diskInput.View() + "\n")
		if m.validationErr != nil {
			s.WriteString(fmt.Sprintf("\n%s\n", errorStyleTUI.Render(fmt.Sprintf("✗ %v", m.validationErr))))
		}

	case stepConfirmation:
		s.WriteString("Review your configuration:\n")

		var panel strings.Builder
		panel.WriteString(m.styles.Label.Render("Template:   ") + utils.Capitalize(m.config.Template) + "\n")
		panel.WriteString(m.styles.Label.Render("GPU Type:   ") + utils.FormatGPUType(m.config.GPUType) + "\n")
		panel.WriteString(m.styles.Label.Render("GPUs:       ") + strconv.Itoa(m.config.NumGPUs) + "\n")
		panel.WriteString(m.styles.Label.Render("vCPUs:      ") + strconv.Itoa(m.config.VCPUs) + "\n")
		confirmRAM := m.specs.RamForVCPU(m.config.GPUType, m.config.NumGPUs, m.config.VCPUs)
		panel.WriteString(m.styles.Label.Render("RAM:        ") + strconv.Itoa(confirmRAM) + " GB\n")
		panel.WriteString(m.styles.Label.Render("Disk Size:  ") + strconv.Itoa(m.config.DiskSizeGB) + " GB")

		s.WriteString(m.styles.Panel.Render(panel.String()))
		s.WriteString("\n")

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

	// Specs and pricing both load asynchronously. Keep the estimated-cost label
	// stable while data is pending, then fill in the value when pricing is ready.
	s.WriteString("\n")
	switch {
	case m.pricing != nil && m.specsLoaded:
		price := m.computePreviewPrice()
		s.WriteString(m.styles.Help.Render(fmt.Sprintf("Estimated cost: %s", utils.FormatPrice(price))))
	case m.pricingLoaded && m.specsLoaded:
		msg := "Estimated cost: unavailable"
		if m.pricingErr != nil {
			msg = fmt.Sprintf("Estimated cost: unavailable (%v)", m.pricingErr)
		}
		s.WriteString(m.styles.Help.Render(msg))
	default:
		s.WriteString(m.styles.Help.Render("Estimated cost:"))
	}

	s.WriteString("\n")
	if m.step == stepConfirmation {
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Confirm  Esc: Back  Q: Quit\n"))
	} else {
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Quit\n"))
	}

	return s.String()
}

// computePreviewPrice calculates the price based on current config state,
// using the hovered option for the current step to preview pricing.
func (m createModel) computePreviewPrice() float64 {
	gpuType := m.config.GPUType
	numGPUs := m.config.NumGPUs
	vcpus := m.config.VCPUs
	diskSizeGB := m.config.DiskSizeGB

	// Apply defaults for unfilled fields
	if gpuType == "" {
		gpuOpts := m.specs.GPUOptions()
		if len(gpuOpts) > 0 {
			gpuType = gpuOpts[0]
		}
	}
	if numGPUs == 0 {
		numGPUs = 1
	}
	if vcpus == 0 {
		vcpus = m.specs.IncludedVCPUs(gpuType, numGPUs)
	}
	if diskSizeGB == 0 {
		diskSizeGB = defaultCreateDiskSizeForGPUCount(numGPUs)
	}

	// Override with hovered option for current step
	switch m.step {
	case stepGPU:
		gpus := m.getGPUOptions()
		if m.cursor < len(gpus) {
			gpuType = gpus[m.cursor]
		}
		if numGPUs == 0 {
			numGPUs = 1
		}
		vcpus = m.specs.IncludedVCPUs(gpuType, numGPUs)
	case stepCompute:
		if m.gpuCountPhase {
			gpuCounts := m.specs.GPUCounts(gpuType)
			if m.cursor < len(gpuCounts) {
				numGPUs = gpuCounts[m.cursor]
			}
			vcpus = m.specs.IncludedVCPUs(gpuType, numGPUs)
		} else {
			vcpuOpts := m.specs.VCPUOptions(m.config.GPUType, m.config.NumGPUs)
			if m.cursor < len(vcpuOpts) {
				vcpus = vcpuOpts[m.cursor]
			}
		}
	case stepDiskSize:
		if v, err := strconv.Atoi(m.diskInput.Value()); err == nil && v >= 10 {
			diskSizeGB = v
		}
	}

	if (m.presets == nil || m.presets.DiskSizeGB == nil) && m.step != stepDiskSize && m.step != stepConfirmation {
		diskSizeGB = max(diskSizeGB, defaultCreateDiskSizeForGPUCount(numGPUs))
	}

	included := m.specs.IncludedVCPUs(gpuType, numGPUs)
	return utils.CalculateHourlyPrice(m.pricing, gpuType, numGPUs, vcpus, diskSizeGB, included)
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
