package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SCPPhase int

const (
	SCPPhaseConnecting SCPPhase = iota
	SCPPhaseCalculatingSize
	SCPPhaseTransferring
	SCPPhaseComplete
	SCPPhaseError
)

type SCPModel struct {
	phase        SCPPhase
	lastPhase    SCPPhase
	direction    string
	currentFile  string
	bytesTotal   int64
	bytesSent    int64
	filesTotal   int
	progress     progress.Model
	spinner      spinner.Model
	speed        float64
	startTime    time.Time
	lastUpdate   time.Time
	lastBytes    int64
	err          error
	quitting     bool
	instanceName string
	logs         []string
	duration     time.Duration
	done         bool
	cancelled    bool

	styles scpStyles
}

type SCPProgressMsg struct {
	BytesSent   int64
	BytesTotal  int64
	CurrentFile string
}

type SCPPhaseMsg struct {
	Phase   SCPPhase
	Message string
}

type SCPCompleteMsg struct {
	FilesTransferred int
	BytesTransferred int64
	Duration         time.Duration
}

type SCPErrorMsg struct {
	Err error
}

type SCPInstanceNameMsg struct {
	InstanceName string
}

type scpStyles struct {
	title      lipgloss.Style
	phase      lipgloss.Style
	log        lipgloss.Style
	logSuccess lipgloss.Style
	file       lipgloss.Style
	stats      lipgloss.Style
	speed      lipgloss.Style
	complete   lipgloss.Style
	successBox lipgloss.Style
	help       lipgloss.Style
}

func newSCPStyles() scpStyles {
	return scpStyles{
		title:      PrimaryTitleStyle(),
		phase:      PrimaryStyle().Italic(true),
		log:        SubtleTextStyle(),
		logSuccess: SuccessStyle().Bold(false),
		file:       PrimaryStyle().Bold(true),
		stats:      SubtleTextStyle(),
		speed:      PrimaryStyle().Bold(true),
		complete:   SuccessStyle(),
		successBox: PrimaryStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(theme.PrimaryColor)).
			Padding(0, 2),
		help: HelpStyle(),
	}
}

func NewSCPModel(direction, instanceName string) SCPModel {
	s := NewPrimarySpinner()
	styles := newSCPStyles()

	p := progress.New(
		progress.WithScaledGradient(theme.PrimaryColor, theme.PrimaryColor),
		progress.WithWidth(60),
	)

	return SCPModel{
		phase:        SCPPhaseConnecting,
		direction:    direction,
		spinner:      s,
		progress:     p,
		startTime:    time.Now(),
		lastUpdate:   time.Now(),
		instanceName: instanceName,
		logs:         []string{"Establishing SSH connection...", "", ""},
		styles:       styles,
	}
}

func (m SCPModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
	)
}

func (m SCPModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "Q" || msg.String() == "ctrl+c" {
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit
		}

	case SCPPhaseMsg:
		if msg.Phase == m.lastPhase {
			return m, nil
		}
		m.lastPhase = msg.Phase
		m.phase = msg.Phase

		switch msg.Phase {
		case SCPPhaseConnecting:
			// Already initialized
		case SCPPhaseCalculatingSize:
			m.logs[0] = "✓ SSH connected"
			m.logs[1] = "Calculating transfer size..."
		case SCPPhaseTransferring:
			m.logs[0] = "✓ SSH connected"
			m.logs[1] = "✓ Size calculated"
			m.logs[2] = "Starting file transfer..."
		}

		return m, nil

	case SCPProgressMsg:
		m.bytesSent = msg.BytesSent
		m.bytesTotal = msg.BytesTotal
		m.currentFile = msg.CurrentFile

		now := time.Now()
		timeDiff := now.Sub(m.lastUpdate).Seconds()
		if timeDiff > 0.5 {
			bytesDiff := float64(m.bytesSent - m.lastBytes)
			m.speed = bytesDiff / timeDiff
			m.lastUpdate = now
			m.lastBytes = m.bytesSent
		}

		return m, nil

	case SCPCompleteMsg:
		m.phase = SCPPhaseComplete
		m.filesTotal = msg.FilesTransferred
		m.bytesTotal = msg.BytesTransferred
		m.bytesSent = msg.BytesTransferred
		m.duration = msg.Duration
		m.logs[0] = "✓ SSH connected"
		m.logs[1] = "✓ Size calculated"
		m.logs[2] = "✓ File transfer completed"
		m.done = true
		m.quitting = true
		return m, deferQuit()

	case SCPErrorMsg:
		m.phase = SCPPhaseError
		m.err = msg.Err
		m.quitting = true
		return m, deferQuit()

	case SCPInstanceNameMsg:
		m.instanceName = msg.InstanceName
		return m, nil

	case quitNow:
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	default:
		return m, nil
	}

	return m, nil
}

func (m SCPModel) View() string {
	if m.cancelled {
		return ""
	}

	var s string

	action := "Upload"
	if m.direction == "download" {
		action = "Download"
	}
	s += m.styles.title.Render(fmt.Sprintf("SCP %s - %s", action, m.instanceName)) + "\n\n"

	s += renderLogs(m)

	switch m.phase {
	case SCPPhaseTransferring:
		s += m.styles.phase.Render("\nTransfer Progress:") + "\n\n"

		if m.currentFile != "" {
			s += m.styles.file.Render("  "+m.currentFile) + "\n\n"
		}

		if m.bytesTotal > 0 {
			percent := float64(m.bytesSent) / float64(m.bytesTotal)
			s += "  " + m.progress.ViewAs(percent) + "\n\n"

			s += m.styles.stats.Render(fmt.Sprintf("  %s / %s",
				formatBytes(m.bytesSent),
				formatBytes(m.bytesTotal))) + "  "

			s += m.styles.stats.Render(fmt.Sprintf("(%.1f%%)", percent*100)) + "\n"

			if m.speed > 0 {
				s += m.styles.speed.Render(fmt.Sprintf("  Speed: %s/s", formatBytes(int64(m.speed)))) + "\n"
			}
		}

	case SCPPhaseComplete:
		s += "\n"
		s += renderSuccessBox(m)
		s += "\n"

	case SCPPhaseError:
		s += errorStyleTUI.Render("✗ Transfer Failed") + "\n\n"
		s += errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	if m.err != nil || m.phase == SCPPhaseError {
		s += errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	s += "\n"
	if m.quitting {
		s += m.styles.help.Render("Closing...\n")
	} else if m.done || m.err != nil {
		s += m.styles.help.Render("Press 'Q' to close\n")
	} else {
		s += m.styles.help.Render("Press 'Q' to cancel\n")
	}

	return s
}

func renderLogs(m SCPModel) string {
	var out string
	for _, line := range m.logs {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "✓") {
			out += m.styles.logSuccess.Render(line) + "\n"
		} else {
			if m.phase == SCPPhaseConnecting && line == "Establishing SSH connection..." {
				out += m.styles.log.Render(fmt.Sprintf("%s %s\n", m.spinner.View(), line))
			} else if m.phase == SCPPhaseCalculatingSize && line == "Calculating transfer size..." {
				out += m.styles.log.Render(fmt.Sprintf("%s %s\n", m.spinner.View(), line))
			} else {
				out += m.styles.log.Render(line + "\n")
			}
		}
	}
	return out
}

func renderSuccessBox(m SCPModel) string {
	direction := "uploaded to"
	if m.direction == "download" {
		direction = "downloaded from"
	}

	lines := []string{
		m.styles.complete.Render("✓ Transfer Complete!"),
		"",
		m.styles.stats.Render(fmt.Sprintf("%-15s %s", "Files "+direction+":", fmt.Sprintf("%d file(s)", m.filesTotal))),
		m.styles.stats.Render(fmt.Sprintf("%-18s %s", "Total size:", formatBytes(m.bytesTotal))),
		m.styles.stats.Render(fmt.Sprintf("%-18s %s", "Duration:", formatDuration(m.duration))),
	}
	if m.duration.Seconds() > 0 {
		avgSpeed := float64(m.bytesTotal) / m.duration.Seconds()
		lines = append(lines, m.styles.stats.Render(fmt.Sprintf("%-18s %s/s", "Average speed:", formatBytes(int64(avgSpeed)))))
	}
	return m.styles.successBox.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func (m SCPModel) Cancelled() bool {
	return m.cancelled
}
