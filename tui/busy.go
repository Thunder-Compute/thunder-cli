package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true).
		MarginTop(1)
)

type BusyDoneMsg struct{}

type BusyModel struct {
	text     string
	spin     spinner.Model
	Quitting bool
}

func NewBusyModel(text string) BusyModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0391ff"))
	return BusyModel{text: text, spin: s}
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
	return "\n  " + m.spin.View() + " " + m.text + "\n\n" + helpStyle.Render("Press q to cancel") + "\n"
}
