package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConnectModel struct {
	instances []string
	cursor    int
	selected  string
	quitting  bool
	done      bool
	cancelled bool
}

var (
	connectCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0391ff"))
	connectSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#50FA7B")).
				Background(lipgloss.Color("#44475A"))
	connectHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Italic(true)
)

func NewConnectModel(instances []string) ConnectModel {
	if instances == nil {
		instances = []string{
			"instance-1 (us-east-1)",
			"instance-2 (us-west-2)",
			"instance-3 (eu-west-1)",
			"instance-4 (ap-south-1)",
		}
	}
	return ConnectModel{
		instances: instances,
	}
}

func (m ConnectModel) Init() tea.Cmd {
	return nil
}

func (m ConnectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "esc", "ctrl+c":
			m.cancelled = true
			m.quitting = true
			return m, deferQuit()

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.instances)-1 {
				m.cursor++
			}

		case "enter":
			m.selected = m.instances[m.cursor]
			m.done = true
			m.quitting = true
			return m, deferQuit()
		}
	case quitNow:
		return m, tea.Quit
	}

	return m, nil
}

func (m ConnectModel) View() string {
	var b strings.Builder

	b.WriteString(connectTitleStyle.Render("⚡ Select Thunder Instance to Connect"))
	b.WriteString("\n\n")

	for i, instance := range m.instances {
		cursor := "  "
		if m.cursor == i {
			cursor = connectCursorStyle.Render("> ")
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, instance))
	}

	if m.done && m.selected != "" {
		b.WriteString("\n")
		b.WriteString(completedStyle.Render(fmt.Sprintf("✓ Selected: %s", m.selected)))
		b.WriteString("\n")
	}
	if m.cancelled {
		b.WriteString("\n")
		b.WriteString(connectWarningStyle.Render("✗ Cancelled"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.quitting {
		b.WriteString(connectHelpStyle.Render("Closing..."))
	} else if m.done || m.cancelled {
		b.WriteString(connectHelpStyle.Render("Press 'Q' to close"))
	} else {
		b.WriteString(connectHelpStyle.Render("↑/↓: Navigate  Enter: Select  Q/Esc: Cancel"))
	}

	return b.String()
}

func RunConnect(instances []string) (string, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	r := lipgloss.NewRenderer(os.Stdout)

	connectSelectedStyle = r.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#50FA7B")).
		Background(lipgloss.Color("#44475A"))

	connectHelpStyle = r.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)

	m := NewConnectModel(instances)
	p := tea.NewProgram(
		m,
		tea.WithContext(ctx),
		tea.WithOutput(os.Stdout),
	)

	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running connect TUI: %w", err)
	}

	if m, ok := finalModel.(ConnectModel); ok {
		if m.cancelled {
			return "", fmt.Errorf("cancelled")
		}
		return m.selected, nil
	}

	return "", nil
}
