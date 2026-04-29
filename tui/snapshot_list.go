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

type snapshotListStyles struct {
	header    lipgloss.Style
	cell      lipgloss.Style
	ready     lipgloss.Style
	creating  lipgloss.Style
	failed    lipgloss.Style
	timestamp lipgloss.Style
}

func newSnapshotListStyles() snapshotListStyles {
	return snapshotListStyles{
		header:    PrimaryTitleStyle().Padding(0, 1),
		cell:      lipgloss.NewStyle().Padding(0, 1),
		ready:     SuccessStyle(),
		creating:  WarningStyle(),
		failed:    ErrorStyle(),
		timestamp: HelpStyle(),
	}
}

type snapshotListModel struct {
	snapshots  api.ListSnapshotsResponse
	client     *api.Client
	monitoring bool
	lastUpdate time.Time
	quitting   bool
	spinner    spinner.Model
	err        error
	cancelled  bool
	styles     snapshotListStyles
}

type snapshotsMsg struct {
	snapshots api.ListSnapshotsResponse
	err       error
}

func newSnapshotListModel(client *api.Client, monitoring bool, snapshots api.ListSnapshotsResponse) snapshotListModel {
	s := NewPrimarySpinner()

	return snapshotListModel{
		client:     client,
		monitoring: monitoring,
		snapshots:  snapshots,
		lastUpdate: time.Now(),
		spinner:    s,
		styles:     newSnapshotListStyles(),
	}
}

func (m snapshotListModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if delay, ok := nextSnapshotPollDelay(m.snapshots); m.monitoring && ok {
		cmds = append(cmds, snapshotsTickCmd(delay))
	}
	return tea.Batch(cmds...)
}

func nextSnapshotPollDelay(snapshots api.ListSnapshotsResponse) (time.Duration, bool) {
	now := time.Now()
	oldestCreatingAge := time.Duration(-1)
	for _, s := range snapshots {
		if s.Status != "CREATING" {
			continue
		}
		if s.CreatedAt <= 0 {
			continue
		}
		age := now.Sub(time.Unix(s.CreatedAt, 0))
		if age > oldestCreatingAge {
			oldestCreatingAge = age
		}
	}
	if oldestCreatingAge < 0 {
		return 60 * time.Second, true
	}
	switch {
	case oldestCreatingAge < 2*time.Minute:
		return 10 * time.Second, true
	case oldestCreatingAge < 10*time.Minute:
		return 20 * time.Second, true
	case oldestCreatingAge < time.Hour:
		return 30 * time.Second, true
	case oldestCreatingAge < 2*time.Hour:
		return 60 * time.Second, true
	default:
		return 5 * time.Minute, true
	}
}

func snapshotsTickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchSnapshotsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		snapshots, err := client.ListSnapshots()
		return snapshotsMsg{snapshots: snapshots, err: err}
	}
}

func (m snapshotListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			return m, deferQuit()
		}
		m.snapshots = msg.snapshots
		m.lastUpdate = time.Now()

		if delay, ok := nextSnapshotPollDelay(m.snapshots); m.monitoring && ok {
			return m, snapshotsTickCmd(delay)
		}

		if !m.monitoring {
			m.quitting = true
			return m, deferQuit()
		}
	}

	return m, nil
}

func (m snapshotListModel) View() string {
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

func (m snapshotListModel) renderTable() string {
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
			m.styles.cell.Width(colWidths["Name"]).Render(name),
			m.styles.cell.Width(colWidths["Status"]).Render(status),
			m.styles.cell.Width(colWidths["Size"]).Render(size),
			m.styles.cell.Width(colWidths["Created"]).Render(created),
		}
		b.WriteString(strings.Join(row, ""))
		b.WriteString("\n")
	}

	return b.String()
}

func (m snapshotListModel) hasCreatingSnapshots() bool {
	for _, s := range m.snapshots {
		if s.Status == "CREATING" {
			return true
		}
	}
	return false
}

func (m snapshotListModel) formatStatus(status string, width int) string {
	var style lipgloss.Style
	switch status {
	case "READY":
		style = m.styles.ready
	case "CREATING":
		style = m.styles.creating
	case "FAILED":
		style = m.styles.failed
	default:
		style = lipgloss.NewStyle()
	}
	return style.Render(truncate(status, width))
}

func RunSnapshotList(client *api.Client, monitoring bool, snapshots api.ListSnapshotsResponse) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	InitCommonStyles(os.Stdout)

	m := newSnapshotListModel(client, monitoring, snapshots)
	p := tea.NewProgram(
		m,
		tea.WithContext(ctx),
		tea.WithOutput(os.Stdout),
	)

	if monitoring {
		disableResizeSignal()
	}

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running snapshot list TUI: %w", err)
	}

	return nil
}
