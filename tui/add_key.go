package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type addKeyStep int

const (
	addKeyStepSelectInstance addKeyStep = iota
	addKeyStepEnterKeyPath
	addKeyStepConfirm
	addKeyStepProgress
	addKeyStepComplete
)

// AddKeyConfig holds the configuration for adding an SSH key
type AddKeyConfig struct {
	InstanceID string
	KeyPath    string
	PublicKey  string
	Confirmed  bool
}

type addKeyModel struct {
	step            addKeyStep
	cursor          int
	config          AddKeyConfig
	instances       []api.Instance
	pathInput       textinput.Model
	suggestions     []string
	selectedSuggest int
	showSuggestions bool
	err             error
	validationErr   error
	quitting        bool
	client          *api.Client
	spinner         spinner.Model
	progressMsg     string

	styles addKeyStyles
}

type addKeyStyles struct {
	title       lipgloss.Style
	selected    lipgloss.Style
	cursor      lipgloss.Style
	panel       lipgloss.Style
	label       lipgloss.Style
	help        lipgloss.Style
	suggestion  lipgloss.Style
	suggestionSelected lipgloss.Style
	keyPreview  lipgloss.Style
}

func newAddKeyStyles() addKeyStyles {
	panelBase := PrimaryStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.PrimaryColor)).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	return addKeyStyles{
		title:       PrimaryTitleStyle().MarginBottom(1),
		selected:    PrimarySelectedStyle(),
		cursor:      PrimaryCursorStyle(),
		panel:       panelBase,
		label:       LabelStyle(),
		help:        HelpStyle(),
		suggestion:  SubtleTextStyle(),
		suggestionSelected: PrimarySelectedStyle(),
		keyPreview:  SubtleTextStyle().MaxWidth(60),
	}
}

func NewAddKeyModel(client *api.Client, instances []api.Instance) addKeyModel {
	styles := newAddKeyStyles()

	// Get default key path
	defaultPath := getDefaultKeyPath()

	ti := textinput.New()
	ti.Placeholder = defaultPath
	ti.SetValue(defaultPath)
	ti.CharLimit = 256
	ti.Width = 50
	ti.Prompt = "▶ "
	ti.PromptStyle = styles.cursor
	ti.TextStyle = styles.cursor
	ti.PlaceholderStyle = SubtleTextStyle()
	ti.Cursor.Style = styles.cursor

	s := NewPrimarySpinner()

	return addKeyModel{
		step:      addKeyStepSelectInstance,
		client:    client,
		instances: instances,
		pathInput: ti,
		spinner:   s,
		styles:    styles,
	}
}

func getDefaultKeyPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/id_rsa.pub"
	}
	return filepath.Join(homeDir, ".ssh", "id_rsa.pub")
}

type addKeyResultMsg struct {
	err error
}

func (m addKeyModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m addKeyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case addKeyResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.step = addKeyStepComplete
		m.config.Confirmed = true
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// Handle key path input
		if m.step == addKeyStepEnterKeyPath && m.pathInput.Focused() {
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.pathInput.Blur()
				m.step = addKeyStepSelectInstance
				m.cursor = 0
				m.validationErr = nil
				m.showSuggestions = false
				return m, nil
			case "enter":
				if m.showSuggestions && len(m.suggestions) > 0 && m.selectedSuggest < len(m.suggestions) {
					// Accept selected suggestion
					m.pathInput.SetValue(m.suggestions[m.selectedSuggest])
					m.pathInput.CursorEnd() // Move cursor to end
					m.showSuggestions = false
					m.suggestions = nil
					return m, nil
				}
				return m.handleEnter()
			case "tab":
				// Autocomplete with first/selected suggestion
				if len(m.suggestions) > 0 {
					if m.selectedSuggest < len(m.suggestions) {
						m.pathInput.SetValue(m.suggestions[m.selectedSuggest])
						m.pathInput.CursorEnd() // Move cursor to end
					}
					// Update suggestions for new path
					m.suggestions = getPathSuggestions(m.pathInput.Value())
					m.selectedSuggest = 0
					m.showSuggestions = len(m.suggestions) > 0
				}
				return m, nil
			case "up":
				if m.showSuggestions && m.selectedSuggest > 0 {
					m.selectedSuggest--
				}
				return m, nil
			case "down":
				if m.showSuggestions && m.selectedSuggest < len(m.suggestions)-1 {
					m.selectedSuggest++
				}
				return m, nil
			default:
				// Pass to text input
				var cmd tea.Cmd
				m.pathInput, cmd = m.pathInput.Update(msg)
				// Update suggestions on input change
				m.suggestions = getPathSuggestions(m.pathInput.Value())
				m.selectedSuggest = 0
				m.showSuggestions = len(m.suggestions) > 0
				if m.validationErr != nil {
					m.validationErr = nil
				}
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.step > addKeyStepSelectInstance && m.step != addKeyStepProgress {
				m.step--
				m.cursor = 0
				m.validationErr = nil
				if m.step == addKeyStepEnterKeyPath {
					m.pathInput.Focus()
				}
			} else if m.step == addKeyStepSelectInstance {
				m.quitting = true
				return m, tea.Quit
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.step != addKeyStepEnterKeyPath && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			maxCursor := m.getMaxCursor()
			if m.step != addKeyStepEnterKeyPath && m.cursor < maxCursor {
				m.cursor++
			}
		}
	}

	return m, nil
}

func (m addKeyModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case addKeyStepSelectInstance:
		if m.cursor < len(m.instances) {
			m.config.InstanceID = m.instances[m.cursor].ID
			m.step = addKeyStepEnterKeyPath
			m.pathInput.Focus()
			// Initialize suggestions
			m.suggestions = getPathSuggestions(m.pathInput.Value())
			m.showSuggestions = false // Don't show by default, only on typing
		}

	case addKeyStepEnterKeyPath:
		path := strings.TrimSpace(m.pathInput.Value())
		if path == "" {
			m.validationErr = fmt.Errorf("key path cannot be empty")
			return m, nil
		}

		// Try to read and validate the key
		publicKey, err := readPublicKey(path)
		if err != nil {
			m.validationErr = err
			return m, nil
		}

		m.config.KeyPath = path
		m.config.PublicKey = publicKey
		m.validationErr = nil
		m.step = addKeyStepConfirm
		m.cursor = 0
		m.pathInput.Blur()
		m.showSuggestions = false

	case addKeyStepConfirm:
		if m.cursor == 0 {
			// Add key
			m.step = addKeyStepProgress
			m.progressMsg = "Adding SSH key to instance..."
			return m, tea.Batch(
				m.spinner.Tick,
				m.addKeyCmd(),
			)
		}
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m addKeyModel) addKeyCmd() tea.Cmd {
	return func() tea.Msg {
		req := &api.AddSSHKeyRequest{
			PublicKey: &m.config.PublicKey,
		}
		_, err := m.client.AddSSHKey(m.config.InstanceID, req)
		return addKeyResultMsg{err: err}
	}
}

func (m addKeyModel) getMaxCursor() int {
	switch m.step {
	case addKeyStepSelectInstance:
		return len(m.instances) - 1
	case addKeyStepConfirm:
		return 1
	}
	return 0
}

func (m addKeyModel) View() string {
	if m.err != nil {
		return ""
	}

	if m.quitting {
		return ""
	}

	if m.step == addKeyStepComplete {
		return ""
	}

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(m.styles.title.Render("🔑 Add SSH Key"))
	s.WriteString("\n\n")

	// Progress indicator
	progressSteps := []string{"Instance", "Key Path", "Confirm"}
	progress := ""
	for i, stepName := range progressSteps {
		adjustedStep := int(m.step)
		if adjustedStep > 2 {
			adjustedStep = 2 // Progress and Complete map to Confirm being done
		}
		if i == adjustedStep {
			progress += m.styles.selected.Render(fmt.Sprintf("[%s]", stepName))
		} else if i < adjustedStep {
			progress += fmt.Sprintf("[✓ %s]", stepName)
		} else {
			progress += fmt.Sprintf("[%s]", stepName)
		}
		if i < len(progressSteps)-1 {
			progress += " → "
		}
	}
	s.WriteString(progress)
	s.WriteString("\n\n")

	switch m.step {
	case addKeyStepSelectInstance:
		s.WriteString("Select a running instance:\n\n")
		for i, instance := range m.instances {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}

			display := fmt.Sprintf("(%s) %s - %sx%s",
				instance.ID,
				instance.Name,
				instance.NumGPUs,
				utils.FormatGPUType(instance.GPUType),
			)
			if m.cursor == i {
				display = m.styles.selected.Render(display)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, display))
		}

	case addKeyStepEnterKeyPath:
		s.WriteString("Enter the path to your SSH public key:\n\n")
		s.WriteString(m.pathInput.View())
		s.WriteString("\n")

		// Show suggestions
		if m.showSuggestions && len(m.suggestions) > 0 {
			s.WriteString("\n")
			s.WriteString(m.styles.label.Render("Suggestions (↑/↓ to select, Tab to complete):"))
			s.WriteString("\n")
			for i, suggestion := range m.suggestions {
				if i == m.selectedSuggest {
					s.WriteString(m.styles.suggestionSelected.Render("  → " + suggestion))
				} else {
					s.WriteString(m.styles.suggestion.Render("    " + suggestion))
				}
				s.WriteString("\n")
			}
		}

		s.WriteString("\n")
		if m.validationErr != nil {
			s.WriteString(errorStyleTUI.Render(fmt.Sprintf("✗ %v", m.validationErr)))
			s.WriteString("\n\n")
		}
		s.WriteString(m.styles.help.Render("Tab: Autocomplete  Enter: Continue  Esc: Back"))

	case addKeyStepConfirm:
		s.WriteString("Review and confirm:\n")

		var panel strings.Builder
		// Find the instance details
		var selectedInstance *api.Instance
		for i := range m.instances {
			if m.instances[i].ID == m.config.InstanceID {
				selectedInstance = &m.instances[i]
				break
			}
		}

		if selectedInstance != nil {
			panel.WriteString(m.styles.label.Render("Instance:  ") + fmt.Sprintf("(%s) %s", selectedInstance.ID, selectedInstance.Name) + "\n")
			panel.WriteString(m.styles.label.Render("IP:        ") + selectedInstance.IP + "\n")
			if selectedInstance.Port != 0 {
				panel.WriteString(m.styles.label.Render("SSH Port:  ") + fmt.Sprintf("%d", selectedInstance.Port) + "\n")
			}
		}
		panel.WriteString(m.styles.label.Render("Key File:  ") + m.config.KeyPath + "\n")

		// Show key preview (truncated)
		keyPreview := m.config.PublicKey
		if len(keyPreview) > 50 {
			keyPreview = keyPreview[:50] + "..."
		}
		panel.WriteString(m.styles.label.Render("Key:       ") + m.styles.keyPreview.Render(keyPreview))

		s.WriteString(m.styles.panel.Render(panel.String()))
		s.WriteString("\n\n")

		s.WriteString("Add this key to the instance?\n\n")
		options := []string{"✓ Add Key", "✗ Cancel"}

		for i, option := range options {
			cursor := "  "
			if m.cursor == i {
				cursor = m.styles.cursor.Render("▶ ")
			}
			text := option
			if m.cursor == i {
				text = m.styles.selected.Render(option)
			}
			s.WriteString(fmt.Sprintf("%s%s\n", cursor, text))
		}

	case addKeyStepProgress:
		s.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), m.progressMsg))
	}

	if m.step != addKeyStepConfirm && m.step != addKeyStepProgress && m.step != addKeyStepEnterKeyPath {
		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Select  Esc: Back  Q: Cancel\n"))
	} else if m.step == addKeyStepConfirm {
		s.WriteString("\n")
		s.WriteString(m.styles.help.Render("↑/↓: Navigate  Enter: Confirm  Q: Cancel\n"))
	}

	return s.String()
}

// Helper functions

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func readPublicKey(path string) (string, error) {
	expandedPath := expandPath(path)

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("key file is empty")
	}

	// Basic validation - check if it looks like an SSH public key
	validPrefixes := []string{
		"ssh-rsa ",
		"ssh-ed25519 ",
		"ssh-dss ",
		"ecdsa-sha2-nistp256 ",
		"ecdsa-sha2-nistp384 ",
		"ecdsa-sha2-nistp521 ",
		"sk-ssh-ed25519@openssh.com ",
		"sk-ecdsa-sha2-nistp256@openssh.com ",
	}

	valid := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(key, prefix) {
			valid = true
			break
		}
	}

	if !valid {
		return "", fmt.Errorf("file does not appear to be a valid SSH public key\nExpected format: ssh-rsa/ssh-ed25519 <key> [comment]")
	}

	return key, nil
}

func getPathSuggestions(input string) []string {
	if input == "" {
		return nil
	}

	expandedInput := expandPath(input)

	// Determine the directory to scan
	dir := expandedInput
	prefix := ""

	stat, err := os.Stat(expandedInput)
	if err != nil || !stat.IsDir() {
		// Not a directory, use parent directory
		dir = filepath.Dir(expandedInput)
		prefix = filepath.Base(expandedInput)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	homeDir, _ := os.UserHomeDir()
	var suggestions []string

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless user is explicitly looking for them
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}

		// Filter by prefix (case-insensitive)
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}

		fullPath := filepath.Join(dir, name)

		// Convert back to use ~ if in home directory
		displayPath := fullPath
		if homeDir != "" && strings.HasPrefix(fullPath, homeDir) {
			displayPath = "~" + fullPath[len(homeDir):]
		}

		// Add trailing slash for directories
		if entry.IsDir() {
			displayPath += "/"
		}

		suggestions = append(suggestions, displayPath)
	}

	// Sort suggestions
	sort.Strings(suggestions)

	// Limit to reasonable number
	if len(suggestions) > 8 {
		suggestions = suggestions[:8]
	}

	return suggestions
}

// RunAddKeyInteractive runs the interactive add-key flow
func RunAddKeyInteractive(client *api.Client, instances []api.Instance) (*api.Instance, error) {
	InitCommonStyles(os.Stdout)
	m := NewAddKeyModel(client, instances)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running TUI: %w", err)
	}

	result := finalModel.(addKeyModel)

	if result.err != nil {
		return nil, result.err
	}

	if result.quitting || !result.config.Confirmed {
		return nil, &CancellationError{}
	}

	// Find and return the selected instance
	for i := range instances {
		if instances[i].ID == result.config.InstanceID {
			return &instances[i], nil
		}
	}

	return nil, fmt.Errorf("instance not found")
}
