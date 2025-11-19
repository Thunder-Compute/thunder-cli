package theme

import (
	"io"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

const (
	PrimaryColorHex     = "#8dc8ff"
	NeutralTextColorHex = "#888888"
	LabelTextColorHex   = "#FFFFFF"
	SuccessColorHex     = "#00D787"
	ErrorColorHex       = "#FF5555"
	WarningColorHex     = "#FFB86C"
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
		primaryStyle = renderer.NewStyle().Foreground(lipgloss.Color(PrimaryColorHex))
		neutralStyle = renderer.NewStyle().Foreground(lipgloss.Color(NeutralTextColorHex))
		labelStyle = renderer.NewStyle().Foreground(lipgloss.Color(LabelTextColorHex)).Bold(true)
		successStyle = renderer.NewStyle().Foreground(lipgloss.Color(SuccessColorHex)).Bold(true)
		errorStyle = renderer.NewStyle().Foreground(lipgloss.Color(ErrorColorHex)).Bold(true)
		warningStyle = renderer.NewStyle().Foreground(lipgloss.Color(WarningColorHex)).Bold(true)
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
