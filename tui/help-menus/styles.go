package helpmenus

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"

	"github.com/Thunder-Compute/thunder-cli/tui/theme"
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

const boxWidth = 77

// Builds a centered box with a title and subtitle. Used in help menus
func HelpHeader(title, subtitle string) string {
	center := func(s string) string {
		pad := (boxWidth - len(s)) / 2
		right := boxWidth - len(s) - pad
		return fmt.Sprintf("│%s%s%s│", strings.Repeat(" ", pad), s, strings.Repeat(" ", right))
	}
	blank := fmt.Sprintf("│%s│", strings.Repeat(" ", boxWidth))
	border := "─────────────────────────────────────────────────────────────────────────────"
	return fmt.Sprintf("\n╭%s╮\n%s\n%s\n%s\n%s\n╰%s╯\n\t", border, blank, center(title), center(subtitle), blank, border)
}

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
