package theme

import (
	"io"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

const (
	PrimaryColor = "12" // Bright Blue
	NeutralColor = "8"  // Bright Black (Gray)
	SuccessColor = "10" // Bright Green
	ErrorColor   = "9"  // Bright Red
	WarningColor = "11" // Bright Yellow
)

var (
	once         sync.Once
	renderer     *lipgloss.Renderer
	primaryStyle lipgloss.Style
	neutralStyle lipgloss.Style
	labelStyle   lipgloss.Style
	successStyle lipgloss.Style
	errorStyle   lipgloss.Style
	warningStyle lipgloss.Style
)

func Init(out io.Writer) {
	once.Do(func() {
		renderer = lipgloss.NewRenderer(out)
		primaryStyle = renderer.NewStyle().Foreground(lipgloss.Color(PrimaryColor))
		neutralStyle = renderer.NewStyle().Foreground(lipgloss.Color(NeutralColor))
		labelStyle = renderer.NewStyle().Bold(true) // Uses terminal default foreground
		successStyle = renderer.NewStyle().Foreground(lipgloss.Color(SuccessColor)).Bold(true)
		errorStyle = renderer.NewStyle().Foreground(lipgloss.Color(ErrorColor)).Bold(true)
		warningStyle = renderer.NewStyle().Foreground(lipgloss.Color(WarningColor)).Bold(true)
	})
}

func Renderer() *lipgloss.Renderer {
	return renderer
}

func Primary() lipgloss.Style {
	return primaryStyle
}

func Neutral() lipgloss.Style {
	return neutralStyle
}

func Label() lipgloss.Style {
	return labelStyle
}

func Success() lipgloss.Style {
	return successStyle
}

func Error() lipgloss.Style {
	return errorStyle
}

func Warning() lipgloss.Style {
	return warningStyle
}
