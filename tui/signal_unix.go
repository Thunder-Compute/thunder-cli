//go:build !windows

package tui

import (
	"os"
	"syscall"
)

// selfInterrupt sends SIGINT to the current process to trigger graceful shutdown.
func selfInterrupt() {
	_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
}
