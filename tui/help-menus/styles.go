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
	flagColorHex    = "#ff6b35"
	descColorHex    = "#f2f2f2"
	exampleColorHex = "#bcbcbc"
)

func InitHelpStyles(out io.Writer) {
	theme.Init(out)

	initOnce.Do(func() {
		r := theme.Renderer()

		HeaderStyle = theme.Primary().Bold(true).Padding(1, 0)
		SectionStyle = theme.Label().MarginTop(1)
		CommandStyle = theme.Primary().Bold(true).Width(20)
		CommandTextStyle = theme.Primary().Bold(true)
		DescStyle = r.NewStyle().Foreground(lipgloss.Color(descColorHex))
		LinkStyle = theme.Label().Underline(true)
		FlagStyle = r.NewStyle().Foreground(lipgloss.Color(flagColorHex)).Bold(true).Width(15)
		ExampleStyle = r.NewStyle().Foreground(lipgloss.Color(exampleColorHex)).Italic(true)
	})
}
