package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joshuawatkins04/thunder-cli-draft/api"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0391ff")).
			Padding(0, 1)

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")) // Green

	startingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")) // Yellow

	deletingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")) // Red

	cellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
)

type StatusModel struct {
	instances   []api.Instance
	client      *api.Client
	monitoring  bool
	lastUpdate  time.Time
	quitting    bool
	spinner     spinner.Model
	err         error
	firstRender bool
}

type tickMsg time.Time

type instancesMsg struct {
	instances []api.Instance
	err       error
}

func NewStatusModel(client *api.Client, monitoring bool, instances []api.Instance) StatusModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0391ff"))

	return StatusModel{
		client:      client,
		monitoring:  monitoring,
		instances:   instances,
		lastUpdate:  time.Now(),
		spinner:     s,
		firstRender: true,
	}
}

func (m StatusModel) Init() tea.Cmd {
	if m.monitoring {
		return tea.Batch(m.spinner.Tick, tickCmd())
	}
	return nil
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

func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}

	case tickMsg:
		// Refresh instances every tick
		return m, tea.Batch(tickCmd(), fetchInstancesCmd(m.client))

	case instancesMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.instances = msg.instances
		m.lastUpdate = time.Now()

		// Check if all instances are stable
		if m.monitoring && m.allInstancesStable() {
			m.quitting = true
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m StatusModel) allInstancesStable() bool {
	if len(m.instances) == 0 {
		return true
	}

	for _, instance := range m.instances {
		if instance.Status == "STARTING" || instance.Status == "DELETING" {
			return false
		}
	}
	return true
}

func (m StatusModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	if m.quitting && !m.monitoring {
		return ""
	}

	var b strings.Builder

	// Build the table
	b.WriteString(m.renderTable())
	b.WriteString("\n")

	// Show timestamp
	if m.monitoring {
		timestamp := m.lastUpdate.Format("15:04:05")
		b.WriteString(timestampStyle.Render(fmt.Sprintf("Last updated: %s", timestamp)))
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString("\n")
	}

	// Instructions
	if m.monitoring {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Press Ctrl+C or q to stop monitoring"))
		b.WriteString("\n")
	}

	return b.String()
}

func (m StatusModel) renderTable() string {
	if len(m.instances) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Render("No instances found. Use 'tnr create' to create a Thunder Compute instance.")
	}

	// Define column widths (content width, padding will be added by styles)
	colWidths := map[string]int{
		"ID":       14,
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

	// Header
	headers := []string{"ID", "Status", "Address", "Mode", "Disk", "GPU", "vCPUs", "RAM", "Template"}
	headerRow := make([]string, len(headers))
	for i, h := range headers {
		headerRow[i] = headerStyle.Copy().Width(colWidths[h]).Render(h)
	}
	b.WriteString(strings.Join(headerRow, ""))
	b.WriteString("\n")

	// Separator
	separatorRow := make([]string, len(headers))
	for i, h := range headers {
		// Account for padding in headerStyle (0, 1) = 2 chars total padding
		separatorRow[i] = strings.Repeat("─", colWidths[h]+2)
	}
	b.WriteString(strings.Join(separatorRow, ""))
	b.WriteString("\n")

	// Rows
	for _, instance := range m.instances {
		// Format fields
		id := truncate(instance.UUID, colWidths["ID"])
		status := m.formatStatus(instance.Status, colWidths["Status"])
		address := truncate(instance.IP, colWidths["Address"])
		mode := truncate(capitalize(instance.Mode), colWidths["Mode"])
		disk := truncate(fmt.Sprintf("%dGB", instance.Storage), colWidths["Disk"])
		gpu := truncate(fmt.Sprintf("%sx%s", instance.NumGPUs, instance.GPUType), colWidths["GPU"])
		vcpus := truncate(instance.CPUCores, colWidths["vCPUs"])
		ram := truncate(fmt.Sprintf("%sGB", instance.Memory), colWidths["RAM"])
		template := truncate(instance.Template, colWidths["Template"])

		row := []string{
			cellStyle.Copy().Width(colWidths["ID"]).Render(id),
			cellStyle.Copy().Width(colWidths["Status"]).Render(status),
			cellStyle.Copy().Width(colWidths["Address"]).Render(address),
			cellStyle.Copy().Width(colWidths["Mode"]).Render(mode),
			cellStyle.Copy().Width(colWidths["Disk"]).Render(disk),
			cellStyle.Copy().Width(colWidths["GPU"]).Render(gpu),
			cellStyle.Copy().Width(colWidths["vCPUs"]).Render(vcpus),
			cellStyle.Copy().Width(colWidths["RAM"]).Render(ram),
			cellStyle.Copy().Width(colWidths["Template"]).Render(template),
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
	case "STARTING":
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

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func RunStatus(client *api.Client, monitoring bool) error {
	// Fetch initial instances
	instances, err := client.ListInstances()
	if err != nil {
		return err
	}

	// If no instances and not monitoring, just print message
	if len(instances) == 0 && !monitoring {
		fmt.Println("No instances found. Use 'tnr create' to create a Thunder Compute instance.")
		return nil
	}

	m := NewStatusModel(client, monitoring, instances)
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running status TUI: %w", err)
	}

	return nil
}
