package tui

import (
	"io"

	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

var (
	helpStyleTUI    lipgloss.Style
	errorStyleTUI   lipgloss.Style
	warningStyleTUI lipgloss.Style
	successStyle    lipgloss.Style

	primaryStyle         lipgloss.Style
	primaryTitleStyle    lipgloss.Style
	primaryCursorStyle   lipgloss.Style
	primarySelectedStyle lipgloss.Style
	labelStyle           lipgloss.Style
	subtleTextStyle      lipgloss.Style
	durationTextStyle    lipgloss.Style
	warningBoxStyle      lipgloss.Style
)

func InitCommonStyles(out io.Writer) {
	theme.Init(out)

	helpStyleTUI = theme.Neutral().Italic(true)
	errorStyleTUI = theme.Error()
	warningStyleTUI = theme.Warning()
	successStyle = theme.Success()

	primaryStyle = theme.Primary()
	primaryTitleStyle = primaryStyle.Bold(true)
	primaryCursorStyle = primaryStyle
	primarySelectedStyle = primaryTitleStyle
	labelStyle = theme.Label()
	subtleTextStyle = theme.Neutral()
	durationTextStyle = subtleTextStyle.Italic(true)
	warningBoxStyle = warningStyleTUI.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.WarningColorHex)).
		Padding(1, 2)
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

func PrimaryStyle() lipgloss.Style {
	return primaryStyle
}

func PrimaryTitleStyle() lipgloss.Style {
	return primaryTitleStyle
}

func PrimaryCursorStyle() lipgloss.Style {
	return primaryCursorStyle
}

func PrimarySelectedStyle() lipgloss.Style {
	return primarySelectedStyle
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

func WarningStyle() lipgloss.Style {
	return warningStyleTUI
}

func SuccessStyle() lipgloss.Style {
	return successStyle
}

func ErrorStyle() lipgloss.Style {
	return errorStyleTUI
}

func NewPrimarySpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = primaryStyle
	return s
}
