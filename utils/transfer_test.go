package utils

import (
	"context"
	"errors"
	"testing"
)

// TestTransferContextAlreadyCancelled guards the CLI-GO-1H regression:
// when Ctrl-C cancels the context before exec.CommandContext manages to
// fork the subprocess, cmd.Run returns context.Canceled directly (not an
// *exec.ExitError). wrapTransferError must recognise this and return
// ErrTransferCancelled so the caller prints "Transfer cancelled" instead
// of leaking a raw "context canceled" error to the user (and to Sentry).
func TestTransferContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Transfer(ctx, "/tmp/nokey", "127.0.0.1", 22, "/tmp/x", "/tmp/y", true)
	if !errors.Is(err, ErrTransferCancelled) {
		t.Fatalf("expected ErrTransferCancelled, got %T: %v", err, err)
	}
}
