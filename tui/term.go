package tui

import (
	"os"

	termx "github.com/charmbracelet/x/term"
)

// IsInteractive returns true when stdout is a TTY and the session
// is suitable for Bubble Tea TUI rendering. Commands use this to
// decide between interactive TUI and plain-text output paths.
func IsInteractive() bool {
	return termx.IsTerminal(os.Stdout.Fd())
}
