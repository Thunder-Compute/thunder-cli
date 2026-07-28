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
//  1. Text a user has to read uses the terminal's default foreground (bold for
//     emphasis, faint to recede). That pairing is the one the user chose, so it
//     is legible by construction.
//  2. Colour is reserved for meaning that is also spelled out in the text, and
//     for decoration where being washed out costs nothing.
const (
	// PrimaryColor is decoration only: borders, spinners, accents. Bright blue
	// collapses to ~1.2:1 on blue backgrounds such as the Windows PowerShell
	// default (#012456) and macOS Terminal's Ocean, so never use it for text
	// that carries information.
	PrimaryColor = "12" // Bright Blue

	// The non-bright green and yellow are used deliberately. Their bright
	// counterparts (10 and 11) drop to 1.7:1 and 1.3:1 on light backgrounds
	// such as macOS Terminal's Basic profile.
	SuccessColor = "2" // Green
	WarningColor = "3" // Yellow

	// Bright red survives everywhere; the non-bright one falls to 1.4:1 on the
	// Windows PowerShell navy.
	ErrorColor = "9" // Bright Red
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
		// Faint dims the terminal's own foreground rather than picking a grey.
		// Colour 8 is not a grey in every scheme: Solarized maps it to the
		// background itself, which made help text invisible on Solarized Dark.
		neutralStyle = renderer.NewStyle().Faint(true)
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
