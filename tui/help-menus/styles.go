package helpmenus

import (
	"io"
	"sync"

	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

var (
	initOnce         sync.Once
	HeaderStyle      lipgloss.Style
	SectionStyle     lipgloss.Style
	CommandStyle     lipgloss.Style
	CommandTextStyle lipgloss.Style
	DescStyle        lipgloss.Style
	LinkStyle        lipgloss.Style
	FlagStyle        lipgloss.Style
	ExampleStyle     lipgloss.Style
)

const (
	flagColor    = "9" // Bright Red
	exampleColor = "8" // Bright Black (Gray)
)

func InitHelpStyles(out io.Writer) {
	theme.Init(out)

	initOnce.Do(func() {
		r := theme.Renderer()

		HeaderStyle = theme.Primary().Bold(true).Padding(1, 0)
		SectionStyle = theme.Label().MarginTop(1)
		CommandStyle = theme.Primary().Bold(true).Width(20)
		CommandTextStyle = theme.Primary().Bold(true)
		DescStyle = r.NewStyle() // Uses terminal default foreground
		LinkStyle = theme.Label().Underline(true)
		FlagStyle = r.NewStyle().Foreground(lipgloss.Color(flagColor)).Bold(true).Width(15)
		ExampleStyle = r.NewStyle().Foreground(lipgloss.Color(exampleColor)).Italic(true)
	})
}
