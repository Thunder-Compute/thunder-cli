package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConnectModel struct {
	instances   []string
	cursor      int
	selected    string
	quitting    bool
	done        bool
	cancelled   bool
	loading     bool
	spin        spinner.Model
	client      *api.Client
	err         error
	displayToID map[string]string
	noInstances bool

	styles connectStyles
}

type connectStyles struct {
	title    lipgloss.Style
	cursor   lipgloss.Style
	selected lipgloss.Style
	help     lipgloss.Style
}

func newConnectStyles() connectStyles {
	return connectStyles{
		title:    PrimaryTitleStyle().MarginTop(1).MarginBottom(1),
		cursor:   PrimaryCursorStyle(),
		selected: PrimarySelectedStyle(),
		help:     HelpStyle(),
	}
}

func NewConnectModel(instances []string) ConnectModel {
	s := NewPrimarySpinner()
	return ConnectModel{
		instances: instances,
		styles:    newConnectStyles(),
		spin:      s,
	}
}

func NewConnectFetchModel(client *api.Client) ConnectModel {
	s := NewPrimarySpinner()
	return ConnectModel{
		loading:     true,
		spin:        s,
		client:      client,
		displayToID: make(map[string]string),
		styles:      newConnectStyles(),
	}
}

func (m ConnectModel) Init() tea.Cmd {
	if m.loading {
		return tea.Batch(m.spin.Tick, fetchConnectInstancesCmd(m.client))
	}
	return nil
}

type connectInstancesMsg struct {
	instances []api.Instance
	err       error
}

func fetchConnectInstancesCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		instances, err := client.ListInstances()
		return connectInstancesMsg{instances: instances, err: err}
	}
}

func (m ConnectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case connectInstancesMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.quitting = true
			return m, tea.Quit
		}

		var items []string
		for _, inst := range msg.instances {
			if inst.Status == "RUNNING" {
				displayName := fmt.Sprintf("%s (%s) - %s GPU: %s", inst.Name, inst.ID, inst.NumGpus, utils.FormatGPUType(inst.GpuType))
				items = append(items, displayName)
				if m.displayToID == nil {
					m.displayToID = make(map[string]string)
				}
				m.displayToID[displayName] = inst.ID
			}
		}
		if len(items) == 0 {
			m.noInstances = true
			m.quitting = true
			return m, tea.Quit
		}
		m.instances = items
		return m, nil
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case tea.KeyMsg:
		if m.loading {
			switch msg.String() {
			case "q", "Q", "esc", "ctrl+c":
				m.cancelled = true
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}
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

	if m.loading {
		return ""
	}

	if m.noInstances {
		return ""
	}

	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	if m.quitting {
		return ""
	}

	b.WriteString(m.styles.title.Render("⚡ Select Thunder Instance to Connect"))
	b.WriteString("\n")
	b.WriteString("Select an instance to connect to:")
	b.WriteString("\n\n")

	for i, instance := range m.instances {
		cursor := "  "
		if m.cursor == i {
			cursor = m.styles.cursor.Render("▶ ")
		}
		line := instance
		if m.cursor == i {
			line = m.styles.selected.Render(instance)
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, line))
	}

	if m.done && m.selected != "" {
		b.WriteString("\n")
		b.WriteString(successStyle.Render(fmt.Sprintf("✓ Selected: %s", m.selected)))
		b.WriteString("\n")
	}
	if m.cancelled {
		b.WriteString("\n")
		b.WriteString(warningStyleTUI.Render("⚠ Cancelled"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.done || m.cancelled {
		b.WriteString(m.styles.help.Render("Press 'Q' to close"))
	} else {
		b.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Cancel\n"))
	}

	return b.String()
}

func RunConnect(instances []string) (string, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	InitCommonStyles(os.Stdout)

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
			return "", &CancellationError{}
		}
		return m.selected, nil
	}

	return "", nil
}

func RunConnectInteractiveSelect(client *api.Client) (string, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	InitCommonStyles(os.Stdout)

	m := NewConnectFetchModel(client)
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
			return "", &CancellationError{}
		}
		if m.err != nil {
			return "", m.err
		}
		if m.noInstances {
			return "", fmt.Errorf("no running instances")
		}
		if m.displayToID != nil && m.selected != "" {
			if id, ok := m.displayToID[m.selected]; ok {
				return id, nil
			}
		}
		return m.selected, nil
	}

	return "", nil
}

func RunConnectSelectWithInstances(instances []api.Instance) (string, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	InitCommonStyles(os.Stdout)

	var items []string
	displayToID := make(map[string]string)
	for _, inst := range instances {
		if inst.Status == "RUNNING" {
			displayName := fmt.Sprintf("(%s) %s - %s GPU: %s", inst.ID, inst.Name, inst.NumGpus, utils.FormatGPUType(inst.GpuType))
			items = append(items, displayName)
			displayToID[displayName] = inst.ID
		}
	}

	if len(items) == 0 {
		return "", fmt.Errorf("no running instances")
	}

	m := NewConnectModel(items)
	m.displayToID = displayToID

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
			return "", &CancellationError{}
		}
		if m.displayToID != nil && m.selected != "" {
			if id, ok := m.displayToID[m.selected]; ok {
				return id, nil
			}
		}
		return m.selected, nil
	}

	return "", nil
}
