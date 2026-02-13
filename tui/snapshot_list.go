package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Thunder-Compute/thunder-cli/api"
)

type SnapshotListModel struct {
	snapshots    api.ListSnapshotsResponse
	client       *api.Client
	monitoring   bool
	lastUpdate   time.Time
	quitting     bool
	spinner      spinner.Model
	err          error
	cancelled bool
}

type snapshotsMsg struct {
	snapshots api.ListSnapshotsResponse
	err       error
}

func NewSnapshotListModel(client *api.Client, monitoring bool, snapshots api.ListSnapshotsResponse) SnapshotListModel {
	s := NewPrimarySpinner()

	return SnapshotListModel{
		client:       client,
		monitoring:   monitoring,
		snapshots:    snapshots,
		lastUpdate:   time.Now(),
		spinner: s,
	}
}

func (m SnapshotListModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if m.monitoring {
		cmds = append(cmds, snapshotsTickCmd())
	}
	return tea.Batch(cmds...)
}

func snapshotsTickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchSnapshotsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		snapshots, err := client.ListSnapshots()
		return snapshotsMsg{snapshots: snapshots, err: err}
	}
}

func (m SnapshotListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		if m.monitoring {
			return m, fetchSnapshotsCmd(m.client)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case snapshotsMsg:
		if msg.err != nil {
			m.err = msg.err
			m.monitoring = false
			return m, nil
		}
		m.snapshots = msg.snapshots
		m.lastUpdate = time.Now()

		if m.monitoring {
			return m, snapshotsTickCmd()
		}

		m.quitting = true
		return m, deferQuit()
	}

	return m, nil
}

func (m SnapshotListModel) View() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	var b strings.Builder

	b.WriteString(m.renderTable())
	b.WriteString("\n")

	if m.hasCreatingSnapshots() {
		b.WriteString(primaryStyle.Render("Snapshot creation can take anywhere from 10 to 90 minutes."))
		b.WriteString("\n")
		b.WriteString(primaryStyle.Render("You can delete your instance and snapshotting will continue in the background."))
		b.WriteString("\n\n")
	}

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

func (m SnapshotListModel) renderTable() string {
	if len(m.snapshots) == 0 {
		return warningStyleTUI.Render("⚠ No snapshots found. Use 'tnr snapshot create' to create a snapshot.")
	}

	colWidths := map[string]int{
		"Name":    30,
		"Status":  12,
		"Size":    10,
		"Created": 22,
	}

	var b strings.Builder

	headers := []string{"Name", "Status", "Size", "Created"}
	headerRow := make([]string, len(headers))
	for i, h := range headers {
		headerRow[i] = snapshotHeaderStyle.Width(colWidths[h]).Render(h)
	}
	b.WriteString(strings.Join(headerRow, ""))
	b.WriteString("\n")

	separatorRow := make([]string, len(headers))
	for i, h := range headers {
		separatorRow[i] = strings.Repeat("─", colWidths[h]+2)
	}
	b.WriteString(strings.Join(separatorRow, ""))
	b.WriteString("\n")

	snapshots := m.snapshots
	if len(snapshots) > 1 {
		sortedSnapshots := make([]api.Snapshot, len(snapshots))
		copy(sortedSnapshots, snapshots)
		sort.Slice(sortedSnapshots, func(i, j int) bool {
			return sortedSnapshots[i].CreatedAt < sortedSnapshots[j].CreatedAt
		})
		snapshots = sortedSnapshots
	}

	for _, snapshot := range snapshots {
		name := truncate(snapshot.Name, colWidths["Name"])
		status := m.formatStatus(snapshot.Status, colWidths["Status"])
		size := truncate(fmt.Sprintf("%d GB", snapshot.MinimumDiskSizeGB), colWidths["Size"])
		createdTime := time.Unix(snapshot.CreatedAt, 0)
		created := truncate(createdTime.Format("2006-01-02 15:04:05"), colWidths["Created"])

		row := []string{
			snapshotCellStyle.Width(colWidths["Name"]).Render(name),
			snapshotCellStyle.Width(colWidths["Status"]).Render(status),
			snapshotCellStyle.Width(colWidths["Size"]).Render(size),
			snapshotCellStyle.Width(colWidths["Created"]).Render(created),
		}
		b.WriteString(strings.Join(row, ""))
		b.WriteString("\n")
	}

	return b.String()
}

func (m SnapshotListModel) hasCreatingSnapshots() bool {
	for _, s := range m.snapshots {
		if s.Status == "CREATING" {
			return true
		}
	}
	return false
}

func (m SnapshotListModel) formatStatus(status string, width int) string {
	var style lipgloss.Style
	switch status {
	case "READY":
		style = snapshotReadyStyle
	case "CREATING":
		style = snapshotCreatingStyle
	case "FAILED":
		style = snapshotFailedStyle
	default:
		style = lipgloss.NewStyle()
	}
	return style.Render(truncate(status, width))
}

var (
	snapshotHeaderStyle   lipgloss.Style
	snapshotCellStyle     lipgloss.Style
	snapshotReadyStyle    lipgloss.Style
	snapshotCreatingStyle lipgloss.Style
	snapshotFailedStyle   lipgloss.Style
)

func RunSnapshotList(client *api.Client, monitoring bool, snapshots api.ListSnapshotsResponse) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	InitCommonStyles(os.Stdout)

	snapshotHeaderStyle = PrimaryTitleStyle().Padding(0, 1)
	snapshotCellStyle = lipgloss.NewStyle().Padding(0, 1)
	snapshotReadyStyle = SuccessStyle()
	snapshotCreatingStyle = WarningStyle()
	snapshotFailedStyle = ErrorStyle()
	timestampStyle = HelpStyle()

	m := NewSnapshotListModel(client, monitoring, snapshots)
	p := tea.NewProgram(
		m,
		tea.WithContext(ctx),
		tea.WithOutput(os.Stdout),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running snapshot list TUI: %w", err)
	}

	return nil
}
