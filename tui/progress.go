package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// ProgressResultMsg is the standard message returned by API commands
// used with ProgressModel. The Err field indicates success or failure.
type ProgressResultMsg struct {
	Err error
}

// ProgressModel is a generic spinner-based progress model for API operations.
// It shows a spinner while the API command executes, then renders a success
// box via renderSuccess, or returns the error for the caller to handle.
type ProgressModel struct {
	spinner       spinner.Model
	message       string
	done          bool
	err           error
	cancelled     bool
	apiCmd        tea.Cmd
	renderSuccess func() string
}

// NewProgressModel creates a ProgressModel that runs apiCmd while showing a spinner.
// On success, renderSuccess is called to produce the final View output.
func NewProgressModel(message string, apiCmd tea.Cmd, renderSuccess func() string) ProgressModel {
	InitCommonStyles(os.Stdout)
	s := NewPrimarySpinner()
	return ProgressModel{
		spinner:       s,
		message:       message,
		apiCmd:        apiCmd,
		renderSuccess: renderSuccess,
	}
}

func (m ProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.apiCmd)
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.done {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case ProgressResultMsg:
		m.done = true
		m.err = msg.Err
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.done = true
			m.cancelled = true
			return m, tea.Quit
		}

	case tea.QuitMsg:
		return m, nil
	}

	return m, nil
}

func (m ProgressModel) View() string {
	if m.done {
		if m.cancelled || m.err != nil {
			return ""
		}
		return m.renderSuccess()
	}
	return fmt.Sprintf("\n %s %s\n", m.spinner.View(), m.message)
}

// Err returns the error from the API command, if any.
func (m ProgressModel) Err() error {
	return m.err
}

// Cancelled returns true if the user cancelled the operation.
func (m ProgressModel) Cancelled() bool {
	return m.cancelled
}
