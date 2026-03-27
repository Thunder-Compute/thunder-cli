package tui

import (
	"fmt"
	"io"
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
		switch msg.String() {
		case "q", "Q", "esc", "ctrl+c":
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
	return m.spin.View() + " " + m.styles.text.Render(m.text) + "\n" + m.styles.help.Render("Esc/Q: Quit\n")
}

// RunWithBusySpinner shows a spinner while fn executes, then dismisses it.
// In non-interactive mode (no TTY), it skips the TUI spinner and runs fn synchronously.
func RunWithBusySpinner(message string, out io.Writer, fn func() error) error {
	if !IsInteractive() {
		fmt.Fprintf(os.Stderr, "%s\n", message)
		return fn()
	}
	busy := NewBusyModel(message)
	bp := tea.NewProgram(busy, tea.WithOutput(out))
	done := make(chan struct{})
	go func() { _, _ = bp.Run(); close(done) }()
	err := fn()
	bp.Send(BusyDoneMsg{})
	<-done
	return err
}
