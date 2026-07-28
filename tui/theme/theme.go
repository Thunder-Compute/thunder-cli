package theme

import (
	"io"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// Every colour here is an ANSI palette index, not RGB, so the terminal's own
// scheme decides what it looks like. A colour that reads well on one scheme can
// be unreadable on another, and the CLI cannot ask the terminal what its
// palette is without a blocking OSC query. The indices below are the ones that
// stay legible across every scheme in internal/termcontrast, which
// tui.TestRenderedTextStaysLegibleAcrossTerminalPalettes checks on every build.
//
// Two rules keep it that way:
//
//  1. Anything the user would lose information by not seeing uses the
//     terminal's default foreground (bold for emphasis, faint to recede). That
//     pairing is the one the user chose, so it is legible by construction.
//     Note the test is "is it lost if it disappears", not "is it text": a
//     spinner's motion and a box's frame both fail it.
//  2. Colour is reserved for meaning that is also spelled out in the words
//     beside it, which is why only the three semantic colours below remain.
//
// There is deliberately no accent colour. Every candidate collided with some
// real scheme (bright blue is 1.1:1 on macOS Terminal's Ocean and 1.8:1 on the
// Windows PowerShell navy), and every place it was used turned out to carry
// information: list cursors, selected rows, help commands, spinners, box
// frames. The only way to keep a brand colour would be to send RGB and override
// the user's palette outright, which is worse.
//
// The semantic colours pick per background. The bright variants are vivid on a
// dark terminal but wash out on a light one (bright yellow is 1.3:1 on white),
// while the normal variants stay legible on light backgrounds but read as muddy
// on dark. Neither choice works alone, so lipgloss resolves them at render time
// from the background it detected.
//
// None of these may be combined with Bold. Many terminals implement bold by
// promoting the colour to its bright slot rather than by using a bold font
// (conhost always, macOS Terminal's "Use bright colors for bold text", Windows
// Terminal's intenseTextStyle), which would drag the light variants back onto
// the bright ones this avoids. Error is the exception: it is already on the
// bright slot when it matters.
var (
	SuccessColor = lipgloss.AdaptiveColor{Light: "2", Dark: "10"}
	WarningColor = lipgloss.AdaptiveColor{Light: "3", Dark: "11"}
	ErrorColor   = lipgloss.AdaptiveColor{Light: "1", Dark: "9"}
)

var (
	once         sync.Once
	renderer     *lipgloss.Renderer
	neutralStyle lipgloss.Style
	bodyStyle    lipgloss.Style
	labelStyle   lipgloss.Style
	successStyle lipgloss.Style
	errorStyle   lipgloss.Style
	warningStyle lipgloss.Style
)

func Init(out io.Writer) {
	once.Do(func() {
		renderer = lipgloss.NewRenderer(out)
		// Seed the background from the default renderer rather than letting
		// this one query the terminal itself. Bubble Tea's package init already
		// forces that query (tea_init.go, "removed in v2"), so the answer is
		// cached and free here; asking again would risk a second
		// termenv.OSCTimeout, which is five seconds on a terminal that does not
		// reply.
		renderer.SetHasDarkBackground(lipgloss.HasDarkBackground())

		// Faint dims the terminal's own foreground rather than picking a grey.
		// Colour 8 is not a grey in every scheme: Solarized maps it to the
		// background itself, which made help text invisible on Solarized Dark.
		neutralStyle = renderer.NewStyle().Faint(true)
		bodyStyle = renderer.NewStyle()             // Uses terminal default foreground
		labelStyle = renderer.NewStyle().Bold(true) // Uses terminal default foreground
		// No Bold here: see the note on the semantic colours above.
		successStyle = renderer.NewStyle().Foreground(SuccessColor)
		warningStyle = renderer.NewStyle().Foreground(WarningColor)
		errorStyle = renderer.NewStyle().Foreground(ErrorColor)
	})
}

func Renderer() *lipgloss.Renderer {
	return renderer
}

func Neutral() lipgloss.Style {
	return neutralStyle
}

// Body is unemphasised text in the terminal's own foreground. Prefer it over a
// bare lipgloss.NewStyle so the style carries the theme's renderer.
func Body() lipgloss.Style {
	return bodyStyle
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
