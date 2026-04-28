package tty

import (
	"os"

	"golang.org/x/term"
)

// Snapshot captures the current termios of stdin/stdout/stderr (whichever is a
// terminal) and returns a function that restores them. Safe to call when no fd
// is a tty, restore becomes a no-op. Used in tandem with a deferred call from
// the process entry point so a panic inside an interactive TUI cannot strand
// the user's shell in raw mode.
func Snapshot() func() {
	type saved struct {
		fd    int
		state *term.State
	}
	var states []saved
	for _, fd := range []int{int(os.Stdin.Fd()), int(os.Stdout.Fd()), int(os.Stderr.Fd())} {
		if !term.IsTerminal(fd) {
			continue
		}
		st, err := term.GetState(fd)
		if err != nil {
			continue
		}
		states = append(states, saved{fd: fd, state: st})
	}
	return func() {
		for _, s := range states {
			_ = term.Restore(s.fd, s.state)
		}
	}
}
