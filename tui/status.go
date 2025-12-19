package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle    lipgloss.Style
	runningStyle   lipgloss.Style
	startingStyle  lipgloss.Style
	deletingStyle  lipgloss.Style
	cellStyle      lipgloss.Style
	timestampStyle lipgloss.Style
)

type StatusModel struct {
	instances  []api.Instance
	client     *api.Client
	monitoring bool
	lastUpdate time.Time
	quitting   bool
	spinner    spinner.Model
	err        error
	done       bool
	cancelled  bool
}

type tickMsg time.Time

type instancesMsg struct {
	instances []api.Instance
	err       error
}

type quitNow struct{}

func NewStatusModel(client *api.Client, monitoring bool, instances []api.Instance) StatusModel {
	s := NewPrimarySpinner()

	return StatusModel{
		client:     client,
		monitoring: monitoring,
		instances:  instances,
		lastUpdate: time.Now(),
		spinner:    s,
	}
}

func (m StatusModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if m.monitoring {
		cmds = append(cmds, tickCmd())
	}
	return tea.Batch(cmds...)
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
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

func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			m.cancelled = true
			m.quitting = true
			m.monitoring = false
			return m, deferQuit()
		}

	case quitNow:
		return m, tea.Quit

	case tickMsg:
		if m.monitoring && len(m.instances) > 0 {
			return m, tea.Batch(tickCmd(), fetchInstancesCmd(m.client))
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case instancesMsg:
		if msg.err != nil {
			m.err = msg.err
			m.monitoring = false
			return m, nil
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

func (m StatusModel) View() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	var b strings.Builder

	b.WriteString(m.renderTable())
	b.WriteString("\n")

	if m.quitting {
		timestamp := m.lastUpdate.Format("15:04:05")
		b.WriteString(timestampStyle.Render(fmt.Sprintf("Last updated: %s", timestamp)))
		b.WriteString("\n")
		return b.String()
	}

	if m.monitoring {
		ts := m.lastUpdate.Format("15:04:05")
		b.WriteString(timestampStyle.Render(fmt.Sprintf("Last updated: %s", ts)))
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

func (m StatusModel) renderTable() string {
	if len(m.instances) == 0 {
		return warningStyleTUI.Render("⚠ No instances found. Use 'tnr create' to create a Thunder Compute instance.")
	}

	colWidths := map[string]int{
		"ID":       4,
		"Name":     14,
		"Status":   12,
		"Address":  18,
		"Mode":     15,
		"Disk":     8,
		"GPU":      10,
		"vCPUs":    8,
		"RAM":      8,
		"Template": 18,
	}

	var b strings.Builder

	headers := []string{"ID", "Name", "Status", "Address", "Mode", "Disk", "GPU", "vCPUs", "RAM", "Template"}
	headerRow := make([]string, len(headers))
	for i, h := range headers {
		headerRow[i] = headerStyle.Width(colWidths[h]).Render(h)
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
		name := truncate(instance.Name, colWidths["Name"])
		status := m.formatStatus(instance.Status, colWidths["Status"])
		address := truncate(instance.IP, colWidths["Address"])
		mode := truncate(utils.Capitalize(instance.Mode), colWidths["Mode"])
		disk := truncate(fmt.Sprintf("%dGB", instance.Storage), colWidths["Disk"])
		gpu := truncate(fmt.Sprintf("%sx%s", instance.NumGPUs, instance.GPUType), colWidths["GPU"])
		vcpus := truncate(instance.CPUCores, colWidths["vCPUs"])
		ram := truncate(fmt.Sprintf("%sGB", instance.Memory), colWidths["RAM"])
		template := truncate(utils.Capitalize(instance.Template), colWidths["Template"])

		row := []string{
			cellStyle.Width(colWidths["ID"]).Render(id),
			cellStyle.Width(colWidths["Name"]).Render(name),
			cellStyle.Width(colWidths["Status"]).Render(status),
			cellStyle.Width(colWidths["Address"]).Render(address),
			cellStyle.Width(colWidths["Mode"]).Render(mode),
			cellStyle.Width(colWidths["Disk"]).Render(disk),
			cellStyle.Width(colWidths["GPU"]).Render(gpu),
			cellStyle.Width(colWidths["vCPUs"]).Render(vcpus),
			cellStyle.Width(colWidths["RAM"]).Render(ram),
			cellStyle.Width(colWidths["Template"]).Render(template),
		}
		b.WriteString(strings.Join(row, ""))
		b.WriteString("\n")
	}

	return b.String()
}

func (m StatusModel) formatStatus(status string, width int) string {
	var style lipgloss.Style
	switch status {
	case "RUNNING":
		style = runningStyle
	case "STARTING", "SNAPPING", "HYDRATING":
		style = startingStyle
	case "DELETING":
		style = deletingStyle
	default:
		style = lipgloss.NewStyle()
	}
	return style.Render(truncate(status, width))
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

func RunStatus(client *api.Client, monitoring bool, instances []api.Instance) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	InitCommonStyles(os.Stdout)

	headerStyle = PrimaryTitleStyle().Padding(0, 1)

	runningStyle = SuccessStyle()

	startingStyle = WarningStyle()

	deletingStyle = ErrorStyle()

	cellStyle = lipgloss.NewStyle().
		Padding(0, 1)

	timestampStyle = HelpStyle()

	m := NewStatusModel(client, monitoring, instances)
	p := tea.NewProgram(
		m,
		tea.WithContext(ctx),
		tea.WithOutput(os.Stdout),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running status TUI: %w", err)
	}

	return nil
}
