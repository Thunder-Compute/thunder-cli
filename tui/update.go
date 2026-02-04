package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type updateStyles struct {
	title      lipgloss.Style
	version    lipgloss.Style
	arrow      lipgloss.Style
	pmBox      lipgloss.Style
	command    lipgloss.Style
	help       lipgloss.Style
	label      lipgloss.Style
	spinnerMsg lipgloss.Style
}

func newUpdateStyles() updateStyles {
	return updateStyles{
		title:   PrimaryTitleStyle(),
		version: PrimaryStyle().Bold(true),
		arrow:   SubtleTextStyle(),
		pmBox: WarningStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(theme.WarningColor)).
			Padding(1, 2),
		command:    PrimaryStyle().Bold(true),
		help:       HelpStyle(),
		label:      LabelStyle(),
		spinnerMsg: LabelStyle().Bold(false),
	}
}

func RenderUpToDate(version string) string {
	InitCommonStyles(os.Stdout)
	return SuccessStyle().Render(fmt.Sprintf("✓ tnr is already up-to-date (%s)", version))
}

func RenderUpdateAvailable(currentVer, latestVer string) string {
	InitCommonStyles(os.Stdout)
	styles := newUpdateStyles()
	return fmt.Sprintf("%s %s %s %s",
		WarningStyle().Render("⚠ Update available:"),
		styles.version.Render(currentVer),
		styles.arrow.Render("→"),
		styles.version.Render(latestVer))
}

func RenderUpdating(currentVer, latestVer string) string {
	InitCommonStyles(os.Stdout)
	styles := newUpdateStyles()
	return fmt.Sprintf("%s %s %s %s%s",
		PrimaryStyle().Render("Updating tnr from"),
		styles.version.Render(currentVer),
		styles.arrow.Render("to"),
		styles.version.Render(latestVer),
		PrimaryStyle().Render("..."))
}

func RenderPMInstructions(pm, currentVer, latestVer string) string {
	InitCommonStyles(os.Stdout)
	styles := newUpdateStyles()

	var content strings.Builder
	content.WriteString(fmt.Sprintf("Update available: %s → %s\n\n",
		styles.version.Render(currentVer),
		styles.version.Render(latestVer)))

	var pmName, command string
	switch pm {
	case "homebrew":
		pmName = "Homebrew"
		command = "brew update && brew upgrade tnr"
	case "scoop":
		pmName = "Scoop"
		command = "scoop update tnr"
	case "winget":
		pmName = "Windows Package Manager"
		command = "winget upgrade Thunder.tnr"
	default:
		pmName = "a package manager"
		command = "brew upgrade tnr"
	}

	content.WriteString(fmt.Sprintf("This installation is managed by %s.\n", pmName))
	content.WriteString(fmt.Sprintf("Run: %s", styles.command.Render(command)))

	return styles.pmBox.Render(content.String())
}

func RenderUpdateSuccess() string {
	InitCommonStyles(os.Stdout)
	return SuccessStyle().Render("✓ Update completed successfully!")
}

// RenderUpdateStaged returns a message for staged Windows updates
func RenderUpdateStaged() string {
	InitCommonStyles(os.Stdout)
	return SuccessStyle().Render("✓ Update staged successfully. Please re-run your command to complete the update.")
}

func RenderUpdateRerun() string {
	InitCommonStyles(os.Stdout)
	return SuccessStyle().Render("✓ Update completed successfully. Please re-run your command.")
}

func RenderUpdateFailed(err error, releaseURL string) string {
	InitCommonStyles(os.Stdout)
	var content strings.Builder
	content.WriteString(ErrorStyle().Render(fmt.Sprintf("✗ Update failed: %v", err)))
	content.WriteString("\n")
	content.WriteString(HelpStyle().Render(fmt.Sprintf("You can download the latest version from: %s", releaseURL)))
	return content.String()
}

type UpdateProgressModel struct {
	spinner  spinner.Model
	message  string
	quitting bool
	done     bool
	err      error
	action   func() error
	styles   updateStyles
}

type updateDoneMsg struct {
	err error
}

func runUpdateAction(action func() error) tea.Cmd {
	return func() tea.Msg {
		err := action()
		return updateDoneMsg{err: err}
	}
}

func NewUpdateProgressModel(message string, action func() error) UpdateProgressModel {
	InitCommonStyles(os.Stdout)
	s := NewPrimarySpinner()
	return UpdateProgressModel{
		spinner: s,
		message: message,
		action:  action,
		styles:  newUpdateStyles(),
	}
}

func (m UpdateProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runUpdateAction(m.action))
}

func (m UpdateProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case updateDoneMsg:
		m.done = true
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
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

func (m UpdateProgressModel) View() string {
	if m.quitting || m.done {
		return ""
	}
	return fmt.Sprintf("%s %s\n", m.spinner.View(), m.styles.spinnerMsg.Render(m.message))
}

func RunUpdateProgress(message string, action func() error) error {
	InitCommonStyles(os.Stdout)
	m := NewUpdateProgressModel(message, action)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running update progress: %w", err)
	}

	result := finalModel.(UpdateProgressModel)
	if result.err != nil {
		return result.err
	}

	return nil
}
