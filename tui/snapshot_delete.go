package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type snapshotDeleteStep int

const (
	snapshotDeleteStepSelect snapshotDeleteStep = iota
	snapshotDeleteStepConfirm
	snapshotDeleteStepComplete
)

type snapshotDeleteModel struct {
	step      snapshotDeleteStep
	cursor    int
	snapshots api.ListSnapshotsResponse
	selected  *api.Snapshot
	confirmed bool
	quitting  bool
	client    *api.Client
	spinner   spinner.Model
	err       error

	styles deleteStyles
}

func NewSnapshotDeleteModel(client *api.Client, snapshots api.ListSnapshotsResponse) snapshotDeleteModel {
	s := NewPrimarySpinner()

	return snapshotDeleteModel{
		step:      snapshotDeleteStepSelect,
		client:    client,
		spinner:   s,
		snapshots: snapshots,
		styles:    newDeleteStyles(),
	}
}

func (m snapshotDeleteModel) Init() tea.Cmd {
	return nil
}

func (m snapshotDeleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.step == snapshotDeleteStepConfirm {
				m.step = snapshotDeleteStepSelect
				m.cursor = 0
			} else {
				m.quitting = true
				return m, tea.Quit
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
		}
	}

	return m, nil
}

func (m snapshotDeleteModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case snapshotDeleteStepSelect:
		if m.cursor < len(m.snapshots) {
			m.selected = &m.snapshots[m.cursor]
			m.step = snapshotDeleteStepConfirm
			m.cursor = 0
		}

	case snapshotDeleteStepConfirm:
		if m.cursor == 0 {
			m.confirmed = true
			m.step = snapshotDeleteStepComplete
			return m, tea.Quit
		}
		m.step = snapshotDeleteStepSelect
		m.cursor = 0
	}

	return m, nil
}

func (m snapshotDeleteModel) getMaxCursor() int {
	switch m.step {
	case snapshotDeleteStepSelect:
		return len(m.snapshots) - 1
	case snapshotDeleteStepConfirm:
		return 1
	}
	return 0
}

func (m snapshotDeleteModel) View() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	if m.quitting {
		return ""
	}

	if m.step == snapshotDeleteStepComplete {
		return ""
	}

	var s strings.Builder

	s.WriteString(m.styles.title.Render("⚡ Delete Snapshot"))
	s.WriteString("\n")

	switch m.step {
	case snapshotDeleteStepSelect:
		s.WriteString("Select a snapshot to delete:\n\n")

		for i, snapshot := range m.snapshots {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}

			// Determine status style
			var statusStyle lipgloss.Style
			status := snapshot.Status
			switch status {
			case "READY":
				statusStyle = SuccessStyle()
			case "CREATING":
				statusStyle = WarningStyle()
			case "FAILED":
				statusStyle = ErrorStyle()
			default:
				statusStyle = lipgloss.NewStyle()
			}

			createdTime := time.Unix(snapshot.CreatedAt, 0)
			display := fmt.Sprintf("%s - %s - %d GB",
				snapshot.Name,
				createdTime.Format("2006-01-02 15:04"),
				snapshot.MinimumDiskSizeGB,
			)
			if m.cursor == i {
				display = m.styles.selected.Render(display)
			}

			statusText := statusStyle.Render(fmt.Sprintf("(%s)", status))

			s.WriteString(fmt.Sprintf("%s%s %s\n", cursor, display, statusText))
		}

		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  Q: Cancel\n"))

	case snapshotDeleteStepConfirm:
		warning := "WARNING: This action is IRREVERSIBLE!\n\n" +
			"Deleting this snapshot will:\n" +
			"• Permanently destroy the snapshot\n" +
			"• This action CANNOT be undone"
		s.WriteString(m.styles.warningBox.Render(warning))
		s.WriteString("\n\n")

		var snapshotInfo strings.Builder
		snapshotInfo.WriteString(m.styles.label.Render("Name:        ") + m.selected.Name + "\n")
		snapshotInfo.WriteString(m.styles.label.Render("Status:      ") + m.selected.Status + "\n")
		snapshotInfo.WriteString(m.styles.label.Render("Disk Size:   ") + fmt.Sprintf("%d GB", m.selected.MinimumDiskSizeGB) + "\n")
		createdTime := time.Unix(m.selected.CreatedAt, 0)
		snapshotInfo.WriteString(m.styles.label.Render("Created:     ") + createdTime.Format("2006-01-02 15:04:05"))

		s.WriteString(m.styles.instanceBox.Render(snapshotInfo.String()))
		s.WriteString("\n\n")

		s.WriteString("Are you sure you want to delete this snapshot?\n\n")

		options := []string{"✓ Yes, Delete Snapshot", "✗ No, Cancel"}
		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}
			if i == 0 {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, ErrorStyle().Render(option)))
			} else {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
			}
		}

		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Confirm  Esc: Back  Q: Cancel\n"))
	}

	return s.String()
}

func RunSnapshotDeleteInteractive(client *api.Client, snapshots api.ListSnapshotsResponse) (*api.Snapshot, error) {
	InitCommonStyles(os.Stdout)
	m := NewSnapshotDeleteModel(client, snapshots)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(snapshotDeleteModel)

	if result.err != nil {
		return nil, result.err
	}

	if result.quitting {
		return nil, &CancellationError{}
	}

	if !result.confirmed || result.selected == nil {
		return nil, &CancellationError{}
	}

	return result.selected, nil
}

type snapshotDeleteProgressModel struct {
	spinner      spinner.Model
	message      string
	quitting     bool
	success      bool
	successMsg   string
	err          error
	client       *api.Client
	snapshotID   string
	snapshotName string
}

type snapshotDeleteResultMsg struct {
	err error
}

func deleteSnapshotCmd(client *api.Client, snapshotID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteSnapshot(snapshotID)
		return snapshotDeleteResultMsg{err: err}
	}
}

func newSnapshotDeleteProgressModel(client *api.Client, snapshotID, snapshotName, message string) snapshotDeleteProgressModel {
	s := NewPrimarySpinner()
	return snapshotDeleteProgressModel{
		spinner:      s,
		message:      message,
		client:       client,
		snapshotID:   snapshotID,
		snapshotName: snapshotName,
	}
}

func (m snapshotDeleteProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, deleteSnapshotCmd(m.client, m.snapshotID))
}

func (m snapshotDeleteProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case snapshotDeleteResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.quitting = true
			return m, tea.Quit
		}
		m.success = true
		m.successMsg = fmt.Sprintf("Successfully deleted snapshot '%s'", m.snapshotName)
		m.quitting = true
		return m, tea.Quit
	case tea.KeyMsg:
		m.quitting = true
		return m, tea.Quit
	case tea.QuitMsg:
		m.quitting = true
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m snapshotDeleteProgressModel) View() string {
	if m.success {
		return ""
	}
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

func RunSnapshotDeleteProgress(client *api.Client, snapshotID, snapshotName string) (string, error) {
	InitCommonStyles(os.Stdout)

	m := newSnapshotDeleteProgressModel(client, snapshotID, snapshotName, fmt.Sprintf("Deleting snapshot '%s'...", snapshotName))
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running deletion: %w", err)
	}

	result := finalModel.(snapshotDeleteProgressModel)
	if result.err != nil {
		return "", result.err
	}

	if result.success {
		return result.successMsg, nil
	}

	return "", nil
}
