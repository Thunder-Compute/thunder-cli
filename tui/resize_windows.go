//go:build windows

package tui

// disableResizeSignal is a no-op on Windows because Windows does not
// support SIGWINCH.
func disableResizeSignal() {}
