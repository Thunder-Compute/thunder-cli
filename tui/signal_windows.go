//go:build windows

package tui

import (
	"os"
)

// selfInterrupt sends an interrupt signal to the current process to trigger graceful shutdown.
// On Windows, syscall.Kill is not available, so we use os.Process.Signal.
func selfInterrupt() {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return
	}
	_ = p.Signal(os.Interrupt)
}
