//go:build !windows

package utils

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// watchWindowSize listens for SIGWINCH and sends window-change requests to the SSH session.
func watchWindowSize(ctx context.Context, fd int, session *ssh.Session) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sigCh:
			width, height, err := term.GetSize(fd)
			if err == nil {
				session.WindowChange(height, width)
			}
		}
	}
}
