package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type BusyDoneMsg struct{}

type BusyModel struct {
	text        string
	spin        spinner.Model
	Quitting    bool
	Interrupted bool

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
			m.Interrupted = true
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
//
// If the user cancels (q/esc/ctrl+c) it returns context.Canceled immediately
// while fn may still be running, so callers must not read fn's outputs unless
// the returned error is nil. Keep fn read-only: a cancel that races a
// just-completed fn is still reported as cancelled.
func RunWithBusySpinner(message string, out io.Writer, fn func() error) error {
	if !IsInteractive() {
		fmt.Fprintf(os.Stderr, "%s\n", message)
		return fn()
	}

	busy := NewBusyModel(message)

	bp := tea.NewProgram(busy, tea.WithOutput(out), tea.WithoutSignalHandler())
	errCh := make(chan error, 1)
	go func() {
		// A panic in fn would crash the process without unwinding the tty-restore
		// defers in cmd.Execute; convert it to an error, keeping the panic-site stack.
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("busy spinner task panicked: %v\n%s", r, debug.Stack())
				bp.Send(BusyDoneMsg{})
			}
		}()
		errCh <- fn()
		bp.Send(BusyDoneMsg{})
	}()

	// A TUI failure must not fail the command; fall through to fn's result.
	finalModel, _ := bp.Run()
	if m, ok := finalModel.(BusyModel); ok && m.Interrupted {
		return context.Canceled
	}
	return <-errCh
}
