package tui

import (
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type BusyDoneMsg struct{}

type BusyModel struct {
	text     string
	spin     spinner.Model
	Quitting bool

	styles busyStyles
}

type busyStyles struct {
	text lipgloss.Style
	help lipgloss.Style
}

func newBusyStyles() busyStyles {
	return busyStyles{
		text: LabelStyle().Bold(false),
		help: HelpStyle(),
	}
}

func NewBusyModel(text string) BusyModel {
	InitCommonStyles(os.Stdout)
	s := NewPrimarySpinner()
	return BusyModel{
		text:   text,
		spin:   s,
		styles: newBusyStyles(),
	}
}

func (m BusyModel) Init() tea.Cmd {
	return m.spin.Tick
}

func (m BusyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case BusyDoneMsg:
		m.Quitting = true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.Quitting = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m BusyModel) View() string {
	if m.Quitting {
		return ""
	}
	return m.spin.View() + " " + m.styles.text.Render(m.text) + "\n" + m.styles.help.Render("Press 'Q' to cancel\n")
}
