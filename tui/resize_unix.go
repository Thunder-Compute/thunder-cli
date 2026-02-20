//go:build !windows

package tui

import (
	"os/signal"
	"syscall"
	"time"
)

// disableResizeSignal prevents Bubble Tea from receiving SIGWINCH,
// which would cause it to re-render the layout on terminal resize.
// This is called after tea.NewProgram is created but before Run returns,
// so the goroutine waits for Bubble Tea to register its listener first.
func disableResizeSignal() {
	go func() {
		time.Sleep(100 * time.Millisecond)
		signal.Reset(syscall.SIGWINCH)
	}()
}
