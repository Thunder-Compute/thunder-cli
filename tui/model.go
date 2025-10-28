package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the state of the TUI application
type Model struct {
	choices  []string
	cursor   int
	selected map[int]struct{}
	quitting bool
}

// NewModel creates a new TUI model with default state
func NewModel() Model {
	return Model{
		choices:  []string{"Connect", "Start", "Status", "Delete"},
		selected: make(map[int]struct{}),
	}
}

// Init is the first function that will be called
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}

	return m, nil
}

// View renders the UI
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	s := "Thunder CLI - Select an action:\n\n"

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "
		if _, ok := m.selected[i]; ok {
			checked = "x"
		}

		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice)
	}

	s += "\nPress 'Q' to cancel.\n"

	return s
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(NewModel())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	return nil
}
