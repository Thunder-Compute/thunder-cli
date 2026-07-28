package tui

import (
	"io"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Thunder-Compute/thunder-cli/internal/clierr"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
)

// ErrCancelled is returned when the user cancels an interactive TUI flow.
var ErrCancelled = clierr.New("operation cancelled")

// ErrNoRunningInstances is returned when no running instances are available.
var ErrNoRunningInstances = clierr.New("no running instances")

// ErrNoChanges is returned when a modify operation has no changes to apply.
var ErrNoChanges = clierr.New("no changes")

var (
	helpStyleTUI    lipgloss.Style
	errorStyleTUI   lipgloss.Style
	warningStyleTUI lipgloss.Style
	successStyle    lipgloss.Style

	titleStyle        lipgloss.Style
	cursorStyle       lipgloss.Style
	selectedStyle     lipgloss.Style
	labelStyle        lipgloss.Style
	subtleTextStyle   lipgloss.Style
	durationTextStyle lipgloss.Style
	warningBoxStyle   lipgloss.Style
)

var initOnce sync.Once

func InitCommonStyles(out io.Writer) {
	initOnce.Do(func() {
		theme.Init(out)

		helpStyleTUI = theme.Neutral().Italic(true)
		errorStyleTUI = theme.Error()
		warningStyleTUI = theme.Warning()
		successStyle = theme.Success()

		titleStyle = theme.Label()
		cursorStyle = theme.Label()
		selectedStyle = theme.Label()
		labelStyle = theme.Label()
		subtleTextStyle = theme.Neutral()
		durationTextStyle = subtleTextStyle.Italic(true)
		warningBoxStyle = warningStyleTUI.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.WarningColor).
			Padding(1, 2)
	})
}

func RenderWarningSimple(message string) string {
	if message == "" {
		return ""
	}
	return warningStyleTUI.Render("⚠ " + message)
}

func RenderWarning(message string) string {
	if message == "" {
		return ""
	}
	return warningStyleTUI.Render("⚠ Warning: " + message)
}

func RenderSuccessSimple(message string) string {
	if message == "" {
		return ""
	}
	return successStyle.Render("✓ " + message)
}

func RenderSuccess(message string) string {
	if message == "" {
		return ""
	}
	return successStyle.Render("✓ Success: " + message)
}

func RenderError(err error) string {
	if err == nil {
		return ""
	}
	return errorStyleTUI.Render("✗ Error: " + err.Error())
}

func RenderErrorMessage(message string) string {
	if message == "" {
		return ""
	}
	return errorStyleTUI.Render("✗ Error: " + message)
}

func TitleStyle() lipgloss.Style {
	return titleStyle
}

func CursorStyle() lipgloss.Style {
	return cursorStyle
}

func SelectedStyle() lipgloss.Style {
	return selectedStyle
}

func LabelStyle() lipgloss.Style {
	return labelStyle
}

func SubtleTextStyle() lipgloss.Style {
	return subtleTextStyle
}

func DurationStyle() lipgloss.Style {
	return durationTextStyle
}

func WarningBoxStyle() lipgloss.Style {
	return warningBoxStyle
}

func HelpStyle() lipgloss.Style {
	return helpStyleTUI
}

func ResetLine(out io.Writer) {
	if out == nil {
		return
	}
	_, _ = io.WriteString(out, "\r\x1b[2K")
}

func ShowCursor(out io.Writer) {
	if out == nil {
		return
	}
	_, _ = io.WriteString(out, "\x1b[?25h")
}

// ShutdownProgram requests a Bubble Tea program to quit and waits for it to exit
// before restoring cursor state. The done channel should be closed by the
// goroutine running p.Run().
func ShutdownProgram(p *tea.Program, done <-chan error, out io.Writer) {
	if p != nil {
		go p.Quit()
	}
	if done != nil {
		<-done
	}
	ResetLine(out)
	ShowCursor(out)
}

func WarningStyle() lipgloss.Style {
	return warningStyleTUI
}

func SuccessStyle() lipgloss.Style {
	return successStyle
}

func ErrorStyle() lipgloss.Style {
	return errorStyleTUI
}

// PanelStyles contains the standard set of styles used by most TUI panels.
type PanelStyles struct {
	Title    lipgloss.Style
	Selected lipgloss.Style
	Cursor   lipgloss.Style
	Panel    lipgloss.Style
	Label    lipgloss.Style
	Help     lipgloss.Style
}

// NewPanelStyles creates the standard panel styles shared across TUI views.
func NewPanelStyles() PanelStyles {
	return PanelStyles{
		Title:    TitleStyle().MarginBottom(1),
		Selected: SelectedStyle(),
		Cursor:   CursorStyle(),
		// Accent on the border, default foreground for anything inside it that
		// does not set its own style.
		Panel: theme.Body().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).MarginTop(1).MarginBottom(1),
		Label: LabelStyle(),
		Help:  HelpStyle(),
	}
}

// SpinnerStyle is deliberately not the accent colour. A spinner's motion is the
// only thing telling the user the process is alive rather than hung, so it has
// to be visible on every scheme; the accent drops to 1.1:1 on a blue background
// and takes that signal with it. Borders keep the accent because losing one
// costs nothing but the grouping cue.
func SpinnerStyle() lipgloss.Style {
	return labelStyle
}

func NewPrimarySpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle()
	return s
}
