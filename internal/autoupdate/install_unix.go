//go:build !windows

package autoupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// unixInstaller handles installation on Unix-like systems (macOS, Linux).
type unixInstaller struct{}

// Install implements the installer interface for Unix systems.
func (u unixInstaller) Install(ctx context.Context, exe, newBinary, version string, src Source) error {
	// Unix: replace atomically
	dir := filepath.Dir(exe)
	if !dirWritable(dir) {
		if err := installWithSudo(newBinary, exe, version); err != nil {
			return err
		}
		return nil
	}

	tmpTarget := filepath.Join(dir, ".tnr-tmp")
	if err := copyFile(newBinary, tmpTarget); err != nil {
		return err
	}
	if err := os.Chmod(tmpTarget, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmpTarget, exe); err != nil {
		if isPermissionError(err) {
			if err := installWithSudo(newBinary, exe, version); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("install failed: %w", err)
	}
	fmt.Printf("Successfully updated to version %s\n", version)
	return nil
}

func installWithSudo(newBinary, exe, version string) error {
	tempDir := filepath.Dir(newBinary)
	tmpFile, err := os.CreateTemp(tempDir, "tnr-sudo-*")
	if err != nil {
		return err
	}
	tmpTarget := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpTarget)
		return err
	}
	defer os.Remove(tmpTarget)

	if err := copyFile(newBinary, tmpTarget); err != nil {
		return err
	}
	if err := os.Chmod(tmpTarget, 0o755); err != nil {
		return err
	}

	cmd := exec.Command("sudo", "mv", tmpTarget, exe)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo install failed: %w", err)
	}
	fmt.Printf("Successfully updated to version %s\n", version)
	return nil
}

func isPermissionError(err error) bool {
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			return errno == syscall.EACCES || errno == syscall.EPERM
		}
	}
	return false
}
