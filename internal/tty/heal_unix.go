//go:build linux || darwin

package tty

import (
	"os"

	"golang.org/x/sys/unix"
)

// healTTY detects an inherited-broken termios state on stdin (typical aftermath
// of a previous tnr / TUI process being SIGKILL'd, OOM'd, or having its terminal
// emulator crash mid-render) and restores cooked-mode behavior in place. No-op
// when the tty is already sane or stdin is not a terminal.
//
// We intervene only when the canonical user-prompt bits (OPOST, ICANON, ISIG)
// are missing — no well-behaved program leaves a shell-facing tty in that state.
func Heal() {
	fd := int(os.Stdin.Fd())
	t, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return
	}
	if t.Oflag&unix.OPOST != 0 && t.Lflag&unix.ICANON != 0 && t.Lflag&unix.ISIG != 0 {
		return // tty is sane, do nothing
	}
	// Restore the bits a shell prompt expects. Leave everything else untouched
	// so users with custom termios setups don't get their config clobbered.
	t.Oflag |= unix.OPOST | unix.ONLCR
	t.Lflag |= unix.ICANON | unix.ISIG | unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHONL
	t.Iflag |= unix.ICRNL | unix.BRKINT
	_ = unix.IoctlSetTermios(fd, ioctlSetTermios, t)
}
