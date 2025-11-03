package tui

import (
	"fmt"
	"strings"

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

var (
	loginPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#a3a3a3")).
				MarginBottom(1)

	loginHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true).
			MarginTop(1)

	tokenInputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#0391ff")).
			Padding(0, 1).
			MarginBottom(1)
)

func NewLoginModel(authURL string) *LoginModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0391ff"))

	ti := textinput.New()
	ti.Placeholder = "Enter your Thunder Compute token..."
	ti.CharLimit = 500
	ti.Width = 50
	ti.Focus()

	return &LoginModel{
		state:      LoginStateWaiting,
		authURL:    authURL,
		spinner:    s,
		tokenInput: ti,
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
		b.WriteString(loginPromptStyle.Render("Authenticate with your browser. If this doesn't open automatically, visit:"))
		b.WriteString("\n")
		b.WriteString(loginPromptStyle.Render(m.authURL))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("%s Waiting for browser callback...", m.spinner.View()))
		b.WriteString("\n\n")
		b.WriteString(loginHelpStyle.Render("Or, press 'T' to enter a token manually."))
		b.WriteString("\n\n")
		b.WriteString(loginHelpStyle.Render("Press 'Q' to cancel."))

	case LoginStateTokenInput:
		b.WriteString(loginPromptStyle.Render("Enter your Thunder Compute token:"))
		b.WriteString("\n\n")
		b.WriteString(tokenInputStyle.Render(m.tokenInput.View()))
		b.WriteString("\n\n")
		b.WriteString(loginHelpStyle.Render("Press Enter to submit, 'Esc' to go back"))
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
