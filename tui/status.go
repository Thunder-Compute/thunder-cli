package tui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

const (
	provisioningExpectedDuration = 10 * time.Minute
	emptyPollInterval            = 60 * time.Second
	emptyPollBackoffAfter        = 2 * time.Hour
	emptyPollBackoffInterval     = 5 * time.Minute
)

type statusStyles struct {
	header       lipgloss.Style
	running      lipgloss.Style
	starting     lipgloss.Style
	restoring    lipgloss.Style
	migrating    lipgloss.Style
	deleting     lipgloss.Style
	provisioning lipgloss.Style
	cell         lipgloss.Style
	timestamp    lipgloss.Style
}

func newStatusStyles() statusStyles {
	return statusStyles{
		// Bold on the terminal's default foreground, not the primary blue: the
		// column titles have to be readable, and blue text disappears on blue
		// backgrounds like the Windows PowerShell default.
		header:       LabelStyle().Padding(0, 1),
		running:      SuccessStyle(),
		starting:     WarningStyle(),
		restoring:    WarningStyle(),
		migrating:    WarningStyle(),
		deleting:     ErrorStyle(),
		provisioning: WarningStyle(),
		cell:         theme.Body().Padding(0, 1),
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
	refreshErr   error
	nextRetryAt  time.Time
	done         bool
	cancelled    bool
	progressBars map[string]progress.Model

	transitionStartedAt time.Time
	emptyStartedAt      time.Time

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

	m := statusModel{
		client:       client,
		monitoring:   monitoring,
		verbose:      verbose,
		instances:    instances,
		lastUpdate:   time.Now(),
		spinner:      s,
		progressBars: make(map[string]progress.Model),
		styles:       newStatusStyles(),
	}
	if hasTransitionalInstance(instances) {
		m.transitionStartedAt = time.Now()
	}
	if monitoring && len(instances) == 0 {
		m.emptyStartedAt = time.Now()
	}
	return m
}

func (m statusModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if m.monitoring {
		cmds = append(cmds, m.tickCmd())
	}
	return tea.Batch(cmds...)
}

func hasTransitionalInstance(instances []api.Instance) bool {
	for _, inst := range instances {
		switch inst.Status {
		case "PROVISIONING", "RESTORING", "MIGRATING", "STARTING", "STAGING", "PENDING", "QUEUED", "UNKNOWN", "MODIFYING":
			return true
		}
	}
	return false
}

func nextInstancePollDelay(transitionAge time.Duration) time.Duration {
	switch {
	case transitionAge < 2*time.Minute:
		return 1 * time.Second
	case transitionAge < 10*time.Minute:
		return 10 * time.Second
	case transitionAge < time.Hour:
		return 30 * time.Second
	case transitionAge < 2*time.Hour:
		return 60 * time.Second
	default:
		return 5 * time.Minute
	}
}

func nextEmptyPollDelay(emptyStartedAt time.Time) time.Duration {
	if emptyStartedAt.IsZero() || time.Since(emptyStartedAt) < emptyPollBackoffAfter {
		return emptyPollInterval
	}
	return emptyPollBackoffInterval
}

func (m statusModel) nextStatusPollDelay() time.Duration {
	if len(m.instances) == 0 {
		return nextEmptyPollDelay(m.emptyStartedAt)
	}
	if !m.transitionStartedAt.IsZero() {
		return nextInstancePollDelay(time.Since(m.transitionStartedAt))
	}
	return 60 * time.Second
}

func tickAfter(delay time.Duration) tea.Cmd {
	if delay < time.Second {
		delay = time.Second
	}
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m statusModel) tickCmd() tea.Cmd {
	return tickAfter(m.nextStatusPollDelay())
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

func (m statusModel) statusRefreshRetryDelay(err error) (time.Duration, bool) {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == http.StatusTooManyRequests:
			if apiErr.HasRetryAfter {
				return apiErr.RetryAfter, true
			}
			return 60 * time.Second, true
		case apiErr.StatusCode >= 500:
			return m.nextStatusPollDelay(), true
		default:
			return 0, false
		}
	}

	if errors.Is(err, api.ErrTransport) {
		return m.nextStatusPollDelay(), true
	}

	return 0, false
}

func formatRetryDelay(delay time.Duration) string {
	if delay <= 0 {
		return "now"
	}

	seconds := int((delay + time.Second - 1) / time.Second)
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	remainingSeconds := seconds % 60
	if remainingSeconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm %02ds", minutes, remainingSeconds)
}

func statusRefreshErrorMessage(err error, retryAt time.Time) string {
	delay := time.Until(retryAt)
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
		return fmt.Sprintf("Refresh rate-limited. Retrying in %s.", formatRetryDelay(delay))
	}
	return fmt.Sprintf("Failed to refresh instances. Retrying in %s.", formatRetryDelay(delay))
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
		if m.monitoring {
			return m, fetchInstancesCmd(m.client)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case instancesMsg:
		if msg.err != nil {
			if m.monitoring {
				if delay, ok := m.statusRefreshRetryDelay(msg.err); ok {
					m.refreshErr = msg.err
					m.nextRetryAt = time.Now().Add(delay)
					return m, tickAfter(delay)
				}
			}
			m.err = msg.err
			m.monitoring = false
			return m, deferQuit()
		}
		m.instances = msg.instances
		m.lastUpdate = time.Now()
		m.refreshErr = nil
		m.nextRetryAt = time.Time{}

		if len(m.instances) == 0 {
			m.transitionStartedAt = time.Time{}
			if m.monitoring {
				if m.emptyStartedAt.IsZero() {
					m.emptyStartedAt = time.Now()
				}
				return m, m.tickCmd()
			}
			m.quitting = true
			return m, deferQuit()
		}

		m.emptyStartedAt = time.Time{}

		if hasTransitionalInstance(m.instances) {
			if m.transitionStartedAt.IsZero() {
				m.transitionStartedAt = time.Now()
			}
		} else {
			m.transitionStartedAt = time.Time{}
		}

		if !m.monitoring {
			m.quitting = true
			return m, deferQuit()
		}

		return m, m.tickCmd()
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

	// Render migrating progress section
	migratingSection := m.renderMigratingSection()
	if migratingSection != "" {
		b.WriteString(migratingSection)
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

	if m.refreshErr != nil && !m.nextRetryAt.IsZero() {
		b.WriteString(warningStyleTUI.Render(statusRefreshErrorMessage(m.refreshErr, m.nextRetryAt)))
		b.WriteString("\n")
	}

	if m.monitoring {
		ts := m.lastUpdate.Format("15:04:05")
		statusText := fmt.Sprintf("Last updated: %s", ts)
		if len(m.instances) == 0 {
			statusText = fmt.Sprintf("Last updated: %s; checking every %s", ts, formatRetryDelay(nextEmptyPollDelay(m.emptyStartedAt)))
		}
		b.WriteString(m.styles.timestamp.Render(statusText))
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
		b.WriteString(theme.Body().Render("ℹ Restoring from a snapshot may take about 10 minutes for every 100GB of data\n"))
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
		"Disk":     8,
		"GPU":      14,
		"vCPUs":    8,
		"RAM":      8,
		"Template": 18,
	}

	var b strings.Builder

	headers := []string{"ID", "UUID", "Status", "Address", "Disk", "GPU", "vCPUs", "RAM", "Template"}
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
		disk := truncate(fmt.Sprintf("%dGB", instance.Storage), colWidths["Disk"])
		gpu := truncate(fmt.Sprintf("%sx%s", instance.NumGPUs, utils.FormatGPUType(instance.GPUType)), colWidths["GPU"])
		vcpus := truncate(instance.CPUCores, colWidths["vCPUs"])
		ram := truncate(fmt.Sprintf("%sGB", instance.Memory), colWidths["RAM"])
		template := truncate(utils.Capitalize(instance.Template), colWidths["Template"])

		row := []string{
			m.styles.cell.Width(colWidths["ID"]).Render(id),
			m.styles.cell.Width(colWidths["UUID"]).Render(uuid),
			m.styles.cell.Width(colWidths["Status"]).Render(status),
			m.styles.cell.Width(colWidths["Address"]).Render(address),
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
	case "MIGRATING":
		style = m.styles.migrating
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
	b.WriteString(LabelStyle().Render("Provisioning Instances:"))
	b.WriteString("\n\n")

	gpuTypes := make([]string, 0, len(instancesByGPU))
	for gpuType := range instancesByGPU {
		gpuTypes = append(gpuTypes, gpuType)
	}
	sort.Strings(gpuTypes)

	for _, gpuType := range gpuTypes {
		instances := instancesByGPU[gpuType]
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

func restorationExpectedDuration(instance api.Instance) time.Duration {
	if instance.SnapshotSizeGB > 0 {
		return utils.EstimateInstanceRestorationDurationGB(instance.SnapshotSizeGB)
	}
	return utils.EstimateInstanceRestorationDurationGB(instance.Storage)
}

func (m *statusModel) renderRestoringSection() string {
	// Filter instances with RESTORING status
	var restoringInstances []api.Instance
	for _, instance := range m.instances {
		if instance.Status == "RESTORING" && !instance.ProgressStartedAt.IsZero() {
			restoringInstances = append(restoringInstances, instance)
		}
	}

	if len(restoringInstances) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(LabelStyle().Render("Restoring Instances:"))
	b.WriteString("\n\n")

	for _, instance := range restoringInstances {
		// Use instance ID for restoring progress bars (each instance gets its own)
		progressBarKey := "restoring-" + instance.ID
		m.ensureProgressBar(progressBarKey)
		progressBar := m.progressBars[progressBarKey]

		// Calculate progress using the GetProgress method
		restoringExpectedDuration := restorationExpectedDuration(instance)
		progressPercent := utils.GetProgress(instance.ProgressStartedAt, restoringExpectedDuration)

		// Calculate time remaining
		elapsed := time.Since(instance.ProgressStartedAt)
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

func (m *statusModel) renderMigratingSection() string {
	// Filter instances with MIGRATING status
	var migratingInstances []api.Instance
	for _, instance := range m.instances {
		if instance.Status == "MIGRATING" && !instance.ProgressStartedAt.IsZero() {
			migratingInstances = append(migratingInstances, instance)
		}
	}

	if len(migratingInstances) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(LabelStyle().Render("Migrating Instances:"))
	b.WriteString("\n\n")

	for _, instance := range migratingInstances {
		// Use instance ID for migrating progress bars (each instance gets its own)
		progressBarKey := "migrating-" + instance.ID
		m.ensureProgressBar(progressBarKey)
		progressBar := m.progressBars[progressBarKey]

		// Calculate progress using the GetProgress method
		migratingExpectedDuration := utils.EstimateInstanceMigrationDuration(instance.Storage)
		progressPercent := utils.GetProgress(instance.ProgressStartedAt, migratingExpectedDuration)

		// Calculate time remaining
		elapsed := time.Since(instance.ProgressStartedAt)
		remaining := migratingExpectedDuration - elapsed
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
			int(migratingExpectedDuration.Minutes()),
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
