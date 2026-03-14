package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/internal/sshkeys"
)

// ── SSH Key Add ─────────────────────────────────────────────────────────────

// SSHKeyAddConfig holds the result of the interactive add flow.
type SSHKeyAddConfig struct {
	Name      string
	PublicKey string
}

type sshKeyAddStep int

const (
	sshKeyAddStepSelect sshKeyAddStep = iota
	sshKeyAddStepName
	sshKeyAddStepPasteKey
	sshKeyAddStepConfirm
	sshKeyAddStepComplete
)

type sshKeyAddModel struct {
	step        sshKeyAddStep
	cursor      int
	localKeys   []sshkeys.DetectedKey
	selectedKey *sshkeys.DetectedKey // nil if "Paste key manually"
	pasteManual bool
	nameInput   textinput.Model
	keyInput    textinput.Model
	config      SSHKeyAddConfig
	quitting    bool
	confirmed   bool
	err         error

	styles PanelStyles
}

func newSSHKeyAddModel(localKeys []sshkeys.DetectedKey) sshKeyAddModel {
	styles := NewPanelStyles()

	nameInput := textinput.New()
	nameInput.Placeholder = "my-key"
	nameInput.CharLimit = 64
	nameInput.Width = 40
	nameInput.Prompt = "▶ "
	nameInput.PromptStyle = styles.Cursor
	nameInput.TextStyle = styles.Cursor
	nameInput.PlaceholderStyle = styles.Cursor
	nameInput.Cursor.Style = styles.Cursor

	keyInput := textinput.New()
	keyInput.Placeholder = "ssh-ed25519 AAAA..."
	keyInput.CharLimit = 2048
	keyInput.Width = 60
	keyInput.Prompt = "▶ "
	keyInput.PromptStyle = styles.Cursor
	keyInput.TextStyle = styles.Cursor
	keyInput.PlaceholderStyle = styles.Cursor
	keyInput.Cursor.Style = styles.Cursor

	return sshKeyAddModel{
		step:      sshKeyAddStepSelect,
		localKeys: localKeys,
		nameInput: nameInput,
		keyInput:  keyInput,
		styles:    styles,
	}
}

func (m sshKeyAddModel) Init() tea.Cmd {
	return nil
}

func (m sshKeyAddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			switch m.step {
			case sshKeyAddStepName:
				m.step = sshKeyAddStepSelect
				m.cursor = 0
				m.nameInput.Blur()
			case sshKeyAddStepPasteKey:
				m.step = sshKeyAddStepName
				m.keyInput.Blur()
				m.nameInput.Focus()
			case sshKeyAddStepConfirm:
				if m.pasteManual {
					m.step = sshKeyAddStepPasteKey
					m.keyInput.Focus()
				} else {
					m.step = sshKeyAddStepName
					m.nameInput.Focus()
				}
				m.cursor = 0
			default:
				m.quitting = true
				return m, tea.Quit
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.step == sshKeyAddStepSelect || m.step == sshKeyAddStepConfirm {
				if m.cursor > 0 {
					m.cursor--
				}
			}

		case "down", "j":
			if m.step == sshKeyAddStepSelect {
				max := len(m.localKeys) // +1 for "Paste manually", -1 for 0-indexed
				if m.cursor < max {
					m.cursor++
				}
			} else if m.step == sshKeyAddStepConfirm {
				if m.cursor < 1 {
					m.cursor++
				}
			}

		default:
			if m.step == sshKeyAddStepName {
				var cmd tea.Cmd
				m.nameInput, cmd = m.nameInput.Update(msg)
				return m, cmd
			}
			if m.step == sshKeyAddStepPasteKey {
				var cmd tea.Cmd
				m.keyInput, cmd = m.keyInput.Update(msg)
				return m, cmd
			}
		}
	}

	return m, nil
}

func (m sshKeyAddModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case sshKeyAddStepSelect:
		if m.cursor < len(m.localKeys) {
			// Selected a detected local key
			m.selectedKey = &m.localKeys[m.cursor]
			m.pasteManual = false
			m.nameInput.SetValue(m.selectedKey.Name)
		} else {
			// "Paste key manually"
			m.pasteManual = true
			m.selectedKey = nil
		}
		m.step = sshKeyAddStepName
		m.nameInput.Focus()

	case sshKeyAddStepName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return m, nil
		}
		m.config.Name = name
		m.nameInput.Blur()

		if m.pasteManual {
			m.step = sshKeyAddStepPasteKey
			m.keyInput.Focus()
		} else {
			m.config.PublicKey = m.selectedKey.PublicKey
			m.step = sshKeyAddStepConfirm
			m.cursor = 0
		}

	case sshKeyAddStepPasteKey:
		key := strings.TrimSpace(m.keyInput.Value())
		if key == "" {
			return m, nil
		}
		m.config.PublicKey = key
		m.keyInput.Blur()
		m.step = sshKeyAddStepConfirm
		m.cursor = 0

	case sshKeyAddStepConfirm:
		if m.cursor == 0 {
			m.confirmed = true
			m.step = sshKeyAddStepComplete
			return m, tea.Quit
		}
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m sshKeyAddModel) View() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	if m.quitting || m.step == sshKeyAddStepComplete {
		return ""
	}

	var s strings.Builder

	s.WriteString("\n")
	s.WriteString(m.styles.Title.Render("⚡ Add SSH Key"))
	s.WriteString("\n\n")

	switch m.step {
	case sshKeyAddStepSelect:
		s.WriteString("Select a key source:\n\n")

		// Local keys first
		if len(m.localKeys) > 0 {
			s.WriteString(m.styles.Label.Render("Detected local keys:") + "\n")
			for i, key := range m.localKeys {
				cursor := "  "
				if m.cursor == i {
					cursor = m.styles.Cursor.Render("▶ ")
				}
				display := fmt.Sprintf("%s (%s)", key.Name, key.Path)
				if m.cursor == i {
					display = m.styles.Selected.Render(display)
				}
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, display))
			}
			s.WriteString("\n")
		}

		// "Paste key manually" option last
		pasteIdx := len(m.localKeys)
		cursor := "  "
		if m.cursor == pasteIdx {
			cursor = m.styles.Cursor.Render("▶ ")
		}
		display := "Paste key manually"
		if m.cursor == pasteIdx {
			display = m.styles.Selected.Render(display)
		}
		s.WriteString(fmt.Sprintf("%s%s\n", cursor, display))

		s.WriteString("\n")
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Select  Q: Cancel\n"))

	case sshKeyAddStepName:
		s.WriteString("Enter a name for this key:\n\n")
		s.WriteString(m.nameInput.View())
		s.WriteString("\n\n")
		s.WriteString(m.styles.Help.Render("Enter: Continue  Esc: Back  Q: Cancel\n"))

	case sshKeyAddStepPasteKey:
		s.WriteString("Paste your SSH public key:\n\n")
		s.WriteString(m.keyInput.View())
		s.WriteString("\n\n")
		s.WriteString(m.styles.Help.Render("Enter: Continue  Esc: Back  Q: Cancel\n"))

	case sshKeyAddStepConfirm:
		s.WriteString("Review your SSH key:\n")

		var panel strings.Builder
		panel.WriteString(m.styles.Label.Render("Name:       ") + m.config.Name + "\n")

		// Show key type from public key
		parts := strings.Fields(m.config.PublicKey)
		if len(parts) >= 1 {
			panel.WriteString(m.styles.Label.Render("Key Type:   ") + parts[0])
		}

		s.WriteString(m.styles.Panel.Render(panel.String()))
		s.WriteString("\n")

		s.WriteString("Add this key?\n\n")
		options := []string{"✓ Add Key", "✗ Cancel"}
		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.Cursor.Render("▶ ")
			}
			text := option
			if m.cursor == i {
				text = m.styles.Selected.Render(option)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, text))
		}

		s.WriteString("\n")
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Confirm  Esc: Back  Q: Cancel\n"))
	}

	return s.String()
}

// RunSSHKeyAddInteractive runs the interactive SSH key add TUI and returns the config.
func RunSSHKeyAddInteractive(client *api.Client) (*SSHKeyAddConfig, error) {
	InitCommonStyles(os.Stdout)

	localKeys, err := sshkeys.DetectLocalKeys()
	if err != nil {
		localKeys = nil
	}

	m := newSSHKeyAddModel(localKeys)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result, ok := finalModel.(sshKeyAddModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if result.err != nil {
		return nil, result.err
	}

	if result.quitting || !result.confirmed {
		return nil, ErrCancelled
	}

	return &result.config, nil
}

// ── SSH Key Delete ──────────────────────────────────────────────────────────

type sshKeyDeleteStep int

const (
	sshKeyDeleteStepSelect sshKeyDeleteStep = iota
	sshKeyDeleteStepConfirm
	sshKeyDeleteStepComplete
)

type sshKeyDeleteModel struct {
	step      sshKeyDeleteStep
	cursor    int
	keys      api.SSHKeyListResponse
	selected  *api.SSHKey
	confirmed bool
	quitting  bool
	client    *api.Client
	spinner   spinner.Model
	err       error

	styles     PanelStyles
	warningBox lipgloss.Style
}

func newSSHKeyDeleteModel(client *api.Client, keys api.SSHKeyListResponse) sshKeyDeleteModel {
	s := NewPrimarySpinner()
	ps := NewPanelStyles()
	ps.Title = PrimaryTitleStyle().MarginTop(1).MarginBottom(1)

	return sshKeyDeleteModel{
		step:       sshKeyDeleteStepSelect,
		client:     client,
		spinner:    s,
		keys:       keys,
		styles:     ps,
		warningBox: WarningBoxStyle().MarginTop(1).MarginBottom(1),
	}
}

func (m sshKeyDeleteModel) Init() tea.Cmd {
	return nil
}

func (m sshKeyDeleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.step == sshKeyDeleteStepConfirm {
				m.step = sshKeyDeleteStepSelect
				m.cursor = 0
			} else {
				m.quitting = true
				return m, tea.Quit
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			maxCursor := m.getMaxCursor()
			if m.cursor < maxCursor {
				m.cursor++
			}
		}
	}

	return m, nil
}

func (m sshKeyDeleteModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case sshKeyDeleteStepSelect:
		if m.cursor < len(m.keys) {
			m.selected = &m.keys[m.cursor]
			m.step = sshKeyDeleteStepConfirm
			m.cursor = 0
		}

	case sshKeyDeleteStepConfirm:
		if m.cursor == 0 {
			m.confirmed = true
			m.step = sshKeyDeleteStepComplete
			return m, tea.Quit
		}
		m.step = sshKeyDeleteStepSelect
		m.cursor = 0
	}

	return m, nil
}

func (m sshKeyDeleteModel) getMaxCursor() int {
	switch m.step {
	case sshKeyDeleteStepSelect:
		return len(m.keys) - 1
	case sshKeyDeleteStepConfirm:
		return 1
	}
	return 0
}

func (m sshKeyDeleteModel) View() string {
	if m.err != nil {
		return errorStyleTUI.Render(fmt.Sprintf("✗ Error: %v\n", m.err))
	}

	if m.quitting {
		return ""
	}

	if m.step == sshKeyDeleteStepComplete {
		return ""
	}

	var s strings.Builder

	s.WriteString(m.styles.Title.Render("⚡ Delete SSH Key"))
	s.WriteString("\n\n")

	switch m.step {
	case sshKeyDeleteStepSelect:
		s.WriteString("Select an SSH key to delete:\n\n")

		for i, key := range m.keys {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.Cursor.Render("▶ ")
			}

			createdTime := time.Unix(key.CreatedAt, 0)
			display := fmt.Sprintf("%s - %s - %s",
				key.Name,
				key.Fingerprint,
				createdTime.Format("2006-01-02 15:04"),
			)
			if m.cursor == i {
				display = m.styles.Selected.Render(display)
			}

			s.WriteString(fmt.Sprintf("%s%s\n", cursor, display))
		}

		s.WriteString("\n")
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Select  Q: Cancel\n"))

	case sshKeyDeleteStepConfirm:
		s.WriteString("Are you sure you want to delete this SSH key?\n\n")

		var keyInfo strings.Builder
		keyInfo.WriteString(m.styles.Label.Render("Name:          ") + m.selected.Name + "\n")
		keyInfo.WriteString(m.styles.Label.Render("Fingerprint:   ") + m.selected.Fingerprint + "\n")
		keyInfo.WriteString(m.styles.Label.Render("Key Type:      ") + m.selected.KeyType + "\n")
		createdTime := time.Unix(m.selected.CreatedAt, 0)
		keyInfo.WriteString(m.styles.Label.Render("Created:       ") + createdTime.Format("2006-01-02 15:04:05"))

		s.WriteString(m.styles.Panel.Render(keyInfo.String()))
		s.WriteString("\n\n")

		options := []string{"✓ Yes, Delete Key", "✗ No, Cancel"}
		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.Cursor.Render("▶ ")
			}
			if i == 0 {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, ErrorStyle().Render(option)))
			} else {
				s.WriteString(fmt.Sprintf("%s%s\n", cursor, option))
			}
		}

		s.WriteString("\n")
		s.WriteString(m.styles.Help.Render("↑/↓: Navigate  Enter: Confirm  Esc: Back  Q: Cancel\n"))
	}

	return s.String()
}

// RunSSHKeyDeleteInteractive runs the interactive SSH key delete TUI.
func RunSSHKeyDeleteInteractive(client *api.Client, keys api.SSHKeyListResponse) (*api.SSHKey, error) {
	InitCommonStyles(os.Stdout)
	m := newSSHKeyDeleteModel(client, keys)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result, ok := finalModel.(sshKeyDeleteModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}

	if result.err != nil {
		return nil, result.err
	}

	if result.quitting {
		return nil, ErrCancelled
	}

	if !result.confirmed || result.selected == nil {
		return nil, ErrCancelled
	}

	return result.selected, nil
}

// Progress model for SSH key deletion

type sshKeyDeleteProgressModel struct {
	spinner    spinner.Model
	message    string
	quitting   bool
	success    bool
	successMsg string
	err        error
	client     *api.Client
	keyID      string
	keyName    string
}

type sshKeyDeleteResultMsg struct {
	err error
}

func deleteSSHKeyCmd(client *api.Client, keyID string) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteSSHKey(keyID)
		return sshKeyDeleteResultMsg{err: err}
	}
}

func newSSHKeyDeleteProgressModel(client *api.Client, keyID, keyName, message string) sshKeyDeleteProgressModel {
	s := NewPrimarySpinner()
	return sshKeyDeleteProgressModel{
		spinner: s,
		message: message,
		client:  client,
		keyID:   keyID,
		keyName: keyName,
	}
}

func (m sshKeyDeleteProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, deleteSSHKeyCmd(m.client, m.keyID))
}

func (m sshKeyDeleteProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sshKeyDeleteResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.quitting = true
			return m, tea.Quit
		}
		m.success = true
		m.successMsg = fmt.Sprintf("Successfully deleted SSH key '%s'", m.keyName)
		m.quitting = true
		return m, tea.Quit
	case tea.KeyMsg:
		m.quitting = true
		return m, tea.Quit
	case tea.QuitMsg:
		m.quitting = true
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m sshKeyDeleteProgressModel) View() string {
	if m.success {
		return ""
	}
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

// RunSSHKeyDeleteProgress runs the delete progress spinner and returns success message.
func RunSSHKeyDeleteProgress(client *api.Client, keyID, keyName string) (string, error) {
	InitCommonStyles(os.Stdout)

	m := newSSHKeyDeleteProgressModel(client, keyID, keyName, fmt.Sprintf("Deleting SSH key '%s'...", keyName))
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running deletion: %w", err)
	}

	result, ok := finalModel.(sshKeyDeleteProgressModel)
	if !ok {
		return "", fmt.Errorf("unexpected model type")
	}
	if result.err != nil {
		return "", result.err
	}

	if result.success {
		return result.successMsg, nil
	}

	return "", nil
}
