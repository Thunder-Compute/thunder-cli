package utils

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// WrapAPIError returns a cleaner error message for common network failures.
func WrapAPIError(err error, context string) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if strings.Contains(errStr, "dial tcp") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "no such host") {
		return fmt.Errorf("%s: no internet connection", context)
	}
	return fmt.Errorf("%s: %w", context, err)
}

// Transfer uses rsync on Mac/Linux (with scp fallback), scp on Windows.
// Retries up to 3 times on connection failures.
func Transfer(ctx context.Context, keyFile, ip string, port int, localPath, remotePath string, upload bool) error {
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if runtime.GOOS != "windows" {
			if _, lookErr := exec.LookPath("rsync"); lookErr == nil {
				err = rsyncTransfer(ctx, keyFile, ip, port, localPath, remotePath, upload)
			} else {
				err = scpTransfer(ctx, keyFile, ip, port, localPath, remotePath, upload)
			}
		} else {
			err = scpTransfer(ctx, keyFile, ip, port, localPath, remotePath, upload)
		}
		if err == nil {
			return nil
		}
		// Only retry on connection failures (exit code 255)
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 255 {
			return wrapTransferError(err, upload)
		}
		if attempt < 3 {
			fmt.Printf("Connection failed, retrying (%d/3)...\n", attempt)
		}
	}
	return wrapTransferError(err, upload)
}

func wrapTransferError(err error, upload bool) error {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}

	switch exitErr.ExitCode() {
	case 2, 23:
		if upload {
			return fmt.Errorf("local file not found")
		}
		return fmt.Errorf("remote file not found")
	case 11:
		if upload {
			return fmt.Errorf("remote directory does not exist")
		}
		return fmt.Errorf("local directory does not exist")
	case 255:
		return fmt.Errorf("connection failed: check your internet or instance status")
	default:
		return fmt.Errorf("transfer failed (exit code %d)", exitErr.ExitCode())
	}
}

func rsyncTransfer(ctx context.Context, keyFile, ip string, port int, localPath, remotePath string, upload bool) error {
	remote := fmt.Sprintf("ubuntu@%s:%s", ip, remotePath)
	sshCmd := fmt.Sprintf("ssh -i %s -p %d -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=30", keyFile, port)
	args := []string{"-az", "-q", "-e", sshCmd, localPath, remote}
	if !upload {
		args = []string{"-az", "-q", "-e", sshCmd, remote, localPath}
	}
	cmd := exec.CommandContext(ctx, "rsync", args...)
	return cmd.Run()
}

func scpTransfer(ctx context.Context, keyFile, ip string, port int, localPath, remotePath string, upload bool) error {
	remote := fmt.Sprintf("ubuntu@%s:%s", ip, remotePath)
	args := []string{"-i", keyFile, "-P", fmt.Sprintf("%d", port), "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-o", "LogLevel=ERROR", "-o", "ConnectTimeout=30", "-q", "-r", localPath, remote}
	if !upload {
		args = []string{"-i", keyFile, "-P", fmt.Sprintf("%d", port), "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-o", "LogLevel=ERROR", "-o", "ConnectTimeout=30", "-q", "-r", remote, localPath}
	}
	cmd := exec.CommandContext(ctx, "scp", args...)
	return cmd.Run()
}

// SCPTransfer is deprecated, use Transfer instead.
func SCPTransfer(ctx context.Context, keyFile, ip string, port int, localPath, remotePath string, upload bool) error {
	return Transfer(ctx, keyFile, ip, port, localPath, remotePath, upload)
}
