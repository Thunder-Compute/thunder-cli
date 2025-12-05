package tui

import (
	"fmt"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LoginState int

const (
	LoginStateWaiting LoginState = iota
	LoginStateTokenInput
	LoginStateSuccess
	LoginStateError
	LoginStateCancelled
)

type LoginModel struct {
	state      LoginState
	authURL    string
	spinner    spinner.Model
	tokenInput textinput.Model
	token      string
	err        error
	quitting   bool

	styles loginStyles
}

type LoginSuccessMsg struct {
	Token string
}

type LoginErrorMsg struct {
	Err error
}

type LoginCancelMsg struct{}

type TokenSubmitMsg struct {
	Token string
}

type SwitchToTokenMsg struct{}

type loginStyles struct {
	prompt lipgloss.Style
	help   lipgloss.Style
	input  lipgloss.Style
}

func newLoginStyles() loginStyles {
	return loginStyles{
		prompt: SubtleTextStyle().MarginBottom(1),
		help:   HelpStyle().MarginTop(1),
		input: PrimaryStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(theme.PrimaryColor)).
			Padding(0, 1).
			MarginBottom(1),
	}
}

func NewLoginModel(authURL string) *LoginModel {
	s := NewPrimarySpinner()
	styles := newLoginStyles()

	ti := textinput.New()
	ti.Placeholder = "Enter your Thunder Compute token..."
	ti.CharLimit = 500
	ti.Width = 50
	ti.Focus()
	ti.PromptStyle = PrimaryCursorStyle()
	ti.TextStyle = PrimaryCursorStyle()
	ti.PlaceholderStyle = SubtleTextStyle()
	ti.Cursor.Style = PrimaryCursorStyle()

	return &LoginModel{
		state:      LoginStateWaiting,
		authURL:    authURL,
		spinner:    s,
		tokenInput: ti,
		styles:     styles,
	}
}

func (m LoginModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case LoginStateWaiting:
			switch msg.String() {
			case "esc", "q", "ctrl+c":
				m.state = LoginStateCancelled
				m.quitting = true
				return m, tea.Quit
			case "t", "T":
				m.state = LoginStateTokenInput
				m.tokenInput.Focus()
				return m, nil
			}
		case LoginStateTokenInput:
			switch msg.String() {
			case "esc":
				m.state = LoginStateWaiting
				m.tokenInput.Blur()
				return m, nil
			case "enter":
				if strings.TrimSpace(m.tokenInput.Value()) != "" {
					return m, func() tea.Msg {
						return TokenSubmitMsg{Token: strings.TrimSpace(m.tokenInput.Value())}
					}
				}
			default:
				m.tokenInput, cmd = m.tokenInput.Update(msg)
				return m, cmd
			}
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case LoginSuccessMsg:
		m.state = LoginStateSuccess
		m.token = msg.Token
		m.quitting = true
		return m, tea.Quit

	case LoginErrorMsg:
		m.state = LoginStateError
		m.err = msg.Err
		m.quitting = true
		return m, tea.Quit

	case LoginCancelMsg:
		m.state = LoginStateCancelled
		m.quitting = true
		return m, tea.Quit

	case TokenSubmitMsg:
		return m, func() tea.Msg {
			return LoginSuccessMsg(msg)
		}
	}

	return m, cmd
}

func (m LoginModel) View() string {
	if m.quitting {
		switch m.state {
		case LoginStateSuccess:
			return successStyle.Render("✓ Successfully authenticated with Thunder Compute!")
		case LoginStateError:
			return errorStyleTUI.Render(fmt.Sprintf("✗ Error: Authentication failed: %v", m.err))
		case LoginStateCancelled:
			return ""
		}
	}

	var b strings.Builder

	switch m.state {
	case LoginStateWaiting:
		b.WriteString(m.styles.prompt.Render("Authenticate with your browser. If this doesn't open automatically, visit:"))
		b.WriteString("\n")
		b.WriteString(m.styles.prompt.Render(m.authURL))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%s Waiting for browser callback...", m.spinner.View()))
		b.WriteString("\n")
		b.WriteString(m.styles.help.Render("Or, press 'T' to enter a token manually. Press 'Q' to cancel"))

	case LoginStateTokenInput:
		b.WriteString(m.styles.prompt.Render("Enter your Thunder Compute token:"))
		b.WriteString("\n")
		b.WriteString(m.styles.input.Render(m.tokenInput.View()))
		b.WriteString(m.styles.help.Render("Press Enter to submit, 'Esc' to go back"))
	}

	return b.String()
}

func (m LoginModel) State() LoginState {
	return m.state
}

func (m LoginModel) Token() string {
	if strings.TrimSpace(m.token) != "" {
		return m.token
	}
	return m.tokenInput.Value()
}

func (m LoginModel) Error() error {
	return m.err
}

func SendLoginSuccess(p *tea.Program, token string) {
	if p != nil {
		p.Send(LoginSuccessMsg{Token: token})
	}
}

func SendLoginError(p *tea.Program, err error) {
	if p != nil {
		p.Send(LoginErrorMsg{Err: err})
	}
}

func SendLoginCancel(p *tea.Program) {
	if p != nil {
		p.Send(LoginCancelMsg{})
	}
}
