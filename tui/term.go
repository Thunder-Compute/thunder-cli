package tui

import (
	"os"
	"runtime"

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
	if forceNonInteractive || !termx.IsTerminal(os.Stdout.Fd()) {
		return false
	}
	if runtime.GOOS == "windows" {
		// On Windows, Bubble Tea reads from CONIN$. If stdin isn't a console
		// (redirected, piped, or spawned by a GUI launcher without a real
		// conhost), the read fails with "No process is on the other end of
		// the pipe." Checking stdin is a terminal avoids that.
		return termx.IsTerminal(os.Stdin.Fd())
	}
	// On Unix, Bubble Tea opens /dev/tty for input. Verify it's accessible
	// to avoid "device not configured" errors in sandboxed environments
	// where stdout is a PTY but no controlling terminal exists.
	f, err := os.Open("/dev/tty")
	if err != nil {
		return false
	}
	f.Close()
	return true
}
