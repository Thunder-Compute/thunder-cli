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
	flagColor = "9" // Bright Red
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

		// Everything a reader has to act on stays on the terminal's default
		// foreground. The command column is already distinct by its fixed
		// width, and the runnable examples are the last thing that should be
		// hard to read: the accent colour drops to 1.1:1 on a blue background.
		HeaderStyle = theme.Label().Padding(1, 0)
		SectionStyle = theme.Label().MarginTop(1)
		CommandStyle = theme.Label().Width(20)
		CommandTextStyle = theme.Label()
		DescStyle = theme.Body()
		LinkStyle = theme.Label().Underline(true)
		FlagStyle = r.NewStyle().Foreground(lipgloss.Color(flagColor)).Bold(true).Width(19)
		// Faint rather than colour 8, which Solarized maps to the background.
		ExampleStyle = theme.Neutral().Italic(true)
	})
}
