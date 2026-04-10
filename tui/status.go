package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

const (
	provisioningExpectedDuration = 10 * time.Minute
)

type statusStyles struct {
	header       lipgloss.Style
	running      lipgloss.Style
	starting     lipgloss.Style
	restoring    lipgloss.Style
	deleting     lipgloss.Style
	provisioning lipgloss.Style
	cell         lipgloss.Style
	timestamp    lipgloss.Style
}

func newStatusStyles() statusStyles {
	return statusStyles{
		header:       PrimaryTitleStyle().Padding(0, 1),
		running:      SuccessStyle(),
		starting:     WarningStyle(),
		restoring:    PrimaryStyle().Bold(true),
		deleting:     ErrorStyle(),
		provisioning: WarningStyle(),
		cell:         lipgloss.NewStyle().Padding(0, 1),
		timestamp:    HelpStyle(),
	}
}

type statusModel struct {
	instances    []api.Instance
	client       *api.Client
	monitoring   bool
	verbose      bool
	lastUpdate   time.Time
	quitting     bool
	spinner      spinner.Model
	err          error
	done         bool
	cancelled    bool
	progressBars map[string]progress.Model

	styles statusStyles
}

type tickMsg time.Time

type instancesMsg struct {
	instances []api.Instance
	err       error
}

type quitNow struct{}

func newStatusModel(client *api.Client, monitoring bool, instances []api.Instance, verbose bool) statusModel {
	s := NewPrimarySpinner()

	return statusModel{
		client:       client,
		monitoring:   monitoring,
		verbose:      verbose,
		instances:    instances,
		lastUpdate:   time.Now(),
		spinner:      s,
		progressBars: make(map[string]progress.Model),
		styles:       newStatusStyles(),
	}
}

func (m statusModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if m.monitoring {
		cmds = append(cmds, tickCmd(m.instances))
	}
	return tea.Batch(cmds...)
}

func tickCmd(instances []api.Instance) tea.Cmd {
	interval := 10 * time.Second
	for _, inst := range instances {
		if inst.Status == "PROVISIONING" || inst.Status == "RESTORING" || inst.Status == "UNKNOWN" {
			interval = 5 * time.Second
			break
		}
	}
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchInstancesCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		instances, err := client.ListInstances()
		return instancesMsg{instances: instances, err: err}
	}
}

func deferQuit() tea.Cmd {
	return tea.Tick(1*time.Millisecond, func(time.Time) tea.Msg { return quitNow{} })
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "esc", "ctrl+c":
			m.cancelled = true
			m.quitting = true
			m.monitoring = false
			return m, deferQuit()
		}

	case quitNow:
		return m, tea.Quit

	case tickMsg:
		if m.monitoring && len(m.instances) > 0 {
			return m, tea.Batch(tickCmd(m.instances), fetchInstancesCmd(m.client))
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case instancesMsg:
		if msg.err != nil {
			m.err = msg.err
			m.monitoring = false
			return m, deferQuit()
		}
		m.instances = msg.instances
		m.lastUpdate = time.Now()

		if len(m.instances) == 0 {
			m.monitoring = false
			m.quitting = true
			return m, deferQuit()
		}

		if !m.monitoring {
			m.quitting = true
			return m, deferQuit()
		}
	}

	return m, nil
}

func (m statusModel) View() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	var b strings.Builder

	b.WriteString(m.renderTable())
	b.WriteString("\n")

	// Render provisioning progress section
	provisioningSection := m.renderProvisioningSection()
	if provisioningSection != "" {
		b.WriteString(provisioningSection)
	}

	// Render restoring progress section
	restoringSection := m.renderRestoringSection()
	if restoringSection != "" {
		b.WriteString(restoringSection)
	}

	// Render recent events section
	eventsSection := m.renderEventsSection()
	if eventsSection != "" {
		b.WriteString(eventsSection)
	}

	if m.quitting {
		timestamp := m.lastUpdate.Format("15:04:05")
		b.WriteString(m.styles.timestamp.Render(fmt.Sprintf("Last updated: %s", timestamp)))
		b.WriteString("\n")
		return b.String()
	}

	if m.monitoring {
		ts := m.lastUpdate.Format("15:04:05")
		b.WriteString(m.styles.timestamp.Render(fmt.Sprintf("Last updated: %s", ts)))
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString("\n")
	}

	if m.err != nil {
		b.WriteString(errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err)))
	}
	if m.cancelled {
		b.WriteString(warningStyleTUI.Render("⚠ Cancelled\n"))
	}
	if m.done {
		b.WriteString(successStyle.Render("✓ Done\n"))
	}

	// Check if any instance is restoring and show informational message
	hasRestoring := false
	for _, instance := range m.instances {
		if instance.Status == "RESTORING" {
			hasRestoring = true
			break
		}
	}
	if hasRestoring {
		b.WriteString(primaryStyle.Render("ℹ Restoring from a snapshot may take about 10 minutes for every 100GB of data\n"))
	}

	b.WriteString("\n")
	if m.quitting {
		b.WriteString(helpStyleTUI.Render("Closing...\n"))
	} else if m.monitoring {
		b.WriteString(helpStyleTUI.Render("Press 'Q' to cancel monitoring\n"))
	} else {
		b.WriteString(helpStyleTUI.Render("Press 'Q' to close\n"))
	}

	return b.String()
}

func (m statusModel) renderTable() string {
	if len(m.instances) == 0 {
		return warningStyleTUI.Render("⚠ No instances found. Use 'tnr create' to create a Thunder Compute instance.")
	}

	colWidths := map[string]int{
		"ID":       4,
		"UUID":     15,
		"Status":   14,
		"Address":  18,
		"Mode":     15,
		"Disk":     8,
		"GPU":      14,
		"vCPUs":    8,
		"RAM":      8,
		"Template": 18,
	}

	var b strings.Builder

	headers := []string{"ID", "UUID", "Status", "Address", "Mode", "Disk", "GPU", "vCPUs", "RAM", "Template"}
	headerRow := make([]string, len(headers))
	for i, h := range headers {
		headerRow[i] = m.styles.header.Width(colWidths[h]).Render(h)
	}
	b.WriteString(strings.Join(headerRow, ""))
	b.WriteString("\n")

	separatorRow := make([]string, len(headers))
	for i, h := range headers {
		separatorRow[i] = strings.Repeat("─", colWidths[h]+2)
	}
	b.WriteString(strings.Join(separatorRow, ""))
	b.WriteString("\n")

	instances := m.instances
	if len(instances) > 1 {
		sortedInstances := make([]api.Instance, len(instances))
		copy(sortedInstances, instances)
		sort.Slice(sortedInstances, func(i, j int) bool {
			return sortedInstances[i].ID < sortedInstances[j].ID
		})
		instances = sortedInstances
	}

	for _, instance := range instances {
		id := truncate(instance.ID, colWidths["ID"])
		uuid := truncate(instance.UUID, colWidths["UUID"])
		status := m.formatStatus(instance.Status, colWidths["Status"])
		address := truncate(instance.GetIP(), colWidths["Address"])
		mode := truncate(utils.Capitalize(instance.Mode), colWidths["Mode"])
		disk := truncate(fmt.Sprintf("%dGB", instance.Storage+instance.EphemeralDiskGB), colWidths["Disk"])
		gpu := truncate(fmt.Sprintf("%sx%s", instance.NumGPUs, utils.FormatGPUType(instance.GPUType)), colWidths["GPU"])
		vcpus := truncate(instance.CPUCores, colWidths["vCPUs"])
		ram := truncate(fmt.Sprintf("%sGB", instance.Memory), colWidths["RAM"])
		template := truncate(utils.Capitalize(instance.Template), colWidths["Template"])

		row := []string{
			m.styles.cell.Width(colWidths["ID"]).Render(id),
			m.styles.cell.Width(colWidths["UUID"]).Render(uuid),
			m.styles.cell.Width(colWidths["Status"]).Render(status),
			m.styles.cell.Width(colWidths["Address"]).Render(address),
			m.styles.cell.Width(colWidths["Mode"]).Render(mode),
			m.styles.cell.Width(colWidths["Disk"]).Render(disk),
			m.styles.cell.Width(colWidths["GPU"]).Render(gpu),
			m.styles.cell.Width(colWidths["vCPUs"]).Render(vcpus),
			m.styles.cell.Width(colWidths["RAM"]).Render(ram),
			m.styles.cell.Width(colWidths["Template"]).Render(template),
		}
		b.WriteString(strings.Join(row, ""))
		b.WriteString("\n")
	}

	return b.String()
}

func (m statusModel) formatStatus(status string, width int) string {
	var style lipgloss.Style
	switch status {
	case "RUNNING":
		style = m.styles.running
	case "STARTING", "SNAPPING":
		style = m.styles.starting
	case "RESTORING":
		style = m.styles.restoring
	case "DELETING":
		style = m.styles.deleting
	case "PROVISIONING":
		style = m.styles.provisioning
	default:
		style = lipgloss.NewStyle()
	}
	return style.Render(truncate(status, width))
}

func (m *statusModel) ensureProgressBar(gpuType string) {
	if _, exists := m.progressBars[gpuType]; !exists {
		p := progress.New(
			progress.WithSolidFill("#FFA500"),
			progress.WithWidth(70),
		)
		m.progressBars[gpuType] = p
	}
}

func (m *statusModel) renderProvisioningSection() string {
	// Group instances with PROVISIONING status by GPU type
	instancesByGPU := make(map[string][]api.Instance)
	for _, instance := range m.instances {
		if instance.Status == "PROVISIONING" && !instance.ProvisioningTime.IsZero() {
			instancesByGPU[instance.GPUType] = append(instancesByGPU[instance.GPUType], instance)
		}
	}

	if len(instancesByGPU) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(primaryStyle.Bold(true).Render("Provisioning Instances:"))
	b.WriteString("\n\n")

	for gpuType, instances := range instancesByGPU {
		m.ensureProgressBar(gpuType)
		progressBar := m.progressBars[gpuType]

		// Use the earliest provisioning time from all instances of this GPU type
		earliestTime := instances[0].ProvisioningTime
		for _, instance := range instances[1:] {
			if instance.ProvisioningTime.Before(earliestTime) {
				earliestTime = instance.ProvisioningTime
			}
		}

		// Calculate progress using the GetProgress method
		progressPercent := utils.GetProgress(earliestTime, provisioningExpectedDuration)

		// Calculate time remaining
		elapsed := time.Since(earliestTime)
		remaining := provisioningExpectedDuration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		remainingMinutes := int(remaining.Minutes())
		if remainingMinutes < 1 {
			remainingMinutes = 1
		}

		// Build comma-separated list of instance names
		var names []string
		for _, instance := range instances {
			names = append(names, instance.Name)
		}
		instanceList := strings.Join(names, ", ")

		// Render instance names (grey, unbolded)
		b.WriteString(fmt.Sprintf("  %s\n", SubtleTextStyle().Render(instanceList)))

		// Render progress bar
		b.WriteString(fmt.Sprintf("  %s\n", progressBar.ViewAs(progressPercent)))

		// Render message (compressed)
		message := fmt.Sprintf("  ~%d min total, ~%d min remaining",
			int(provisioningExpectedDuration.Minutes()),
			remainingMinutes,
		)
		b.WriteString(m.styles.timestamp.Render(message))
		b.WriteString("\n\n")
	}

	return b.String()
}

func (m *statusModel) renderRestoringSection() string {
	// Filter instances with RESTORING status
	var restoringInstances []api.Instance
	for _, instance := range m.instances {
		if instance.Status == "RESTORING" && !instance.RestoringTime.IsZero() {
			restoringInstances = append(restoringInstances, instance)
		}
	}

	if len(restoringInstances) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(primaryStyle.Bold(true).Render("Restoring Instances:"))
	b.WriteString("\n\n")

	for _, instance := range restoringInstances {
		// Use instance ID for restoring progress bars (each instance gets its own)
		progressBarKey := "restoring-" + instance.ID
		m.ensureProgressBar(progressBarKey)
		progressBar := m.progressBars[progressBarKey]

		// Calculate progress using the GetProgress method
		restoringExpectedDuration := utils.EstimateInstanceRestorationDuration(instance.SnapshotSize)
		progressPercent := utils.GetProgress(instance.RestoringTime, restoringExpectedDuration)

		// Calculate time remaining
		elapsed := time.Since(instance.RestoringTime)
		remaining := restoringExpectedDuration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		remainingMinutes := int(remaining.Minutes())
		if remainingMinutes < 1 {
			remainingMinutes = 1
		}

		// Render instance name (grey, unbolded)
		b.WriteString(fmt.Sprintf("  %s\n", SubtleTextStyle().Render(instance.Name)))

		// Render progress bar
		b.WriteString(fmt.Sprintf("  %s\n", progressBar.ViewAs(progressPercent)))

		// Render message (compressed)
		message := fmt.Sprintf("  ~%d min total, ~%d min remaining",
			int(restoringExpectedDuration.Minutes()),
			remainingMinutes,
		)
		b.WriteString(m.styles.timestamp.Render(message))
		b.WriteString("\n\n")
	}

	return b.String()
}

func (m *statusModel) renderEventsSection() string {
	// Collect instances with recent events
	type instanceEvent struct {
		id        string
		name      string
		reason    string
		message   string
		timestamp time.Time
	}

	var events []instanceEvent
	for _, instance := range m.instances {
		if instance.LastRestart == nil {
			continue
		}
		if !m.verbose && time.Since(instance.LastRestart.Timestamp) > time.Hour {
			continue
		}
		events = append(events, instanceEvent{
			id:        instance.ID,
			name:      instance.Name,
			reason:    instance.LastRestart.Reason,
			message:   instance.LastRestart.Message,
			timestamp: instance.LastRestart.Timestamp,
		})
	}

	if len(events) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(SubtleTextStyle().Bold(true).Render("Recent Events:"))
	b.WriteString("\n\n")

	for _, ev := range events {
		ts := ev.timestamp.Local().Format("2006-01-02_15:04:05.0000")
		b.WriteString(fmt.Sprintf("  %s\n", SubtleTextStyle().Render(ev.name)))
		b.WriteString(fmt.Sprintf("  %s\n",
			SubtleTextStyle().Render(fmt.Sprintf("[%s]: %s — %s", ts, ev.reason, ev.message)),
		))
		b.WriteString("\n")
	}

	return b.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func RunStatus(client *api.Client, monitoring bool, instances []api.Instance, verbose bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	InitCommonStyles(os.Stdout)

	m := newStatusModel(client, monitoring, instances, verbose)
	p := tea.NewProgram(
		m,
		tea.WithContext(ctx),
		tea.WithOutput(os.Stdout),
	)

	if monitoring {
		disableResizeSignal()
	}

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running status TUI: %w", err)
	}

	return nil
}
