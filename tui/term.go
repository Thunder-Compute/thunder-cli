package tui

import (
	"os"

	termx "github.com/charmbracelet/x/term"
)

var forceNonInteractive bool

// SetNonInteractive disables interactive mode regardless of TTY state.
// Call this when --json or similar flags are set.
func SetNonInteractive(v bool) {
	forceNonInteractive = v
}

// IsInteractive returns true when stdout is a TTY and the session
// is suitable for Bubble Tea TUI rendering. Commands use this to
// decide between interactive TUI and plain-text output paths.
func IsInteractive() bool {
	return !forceNonInteractive && termx.IsTerminal(os.Stdout.Fd())
}
