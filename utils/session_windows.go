//go:build windows

package utils

import (
	"context"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// watchWindowSize polls for terminal size changes and sends window-change requests
// to the SSH session. Windows does not support SIGWINCH, so we poll instead.
func watchWindowSize(ctx context.Context, fd int, session *ssh.Session) {
	prevW, prevH, _ := term.GetSize(fd)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w, h, err := term.GetSize(fd)
			if err == nil && (w != prevW || h != prevH) {
				session.WindowChange(h, w)
				prevW, prevH = w, h
			}
		}
	}
}
