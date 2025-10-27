package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// AcquireLock creates a lock file to prevent concurrent connections
func AcquireLock(instanceID string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	lockDir := filepath.Join(homeDir, ".thunder", "locks")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return fmt.Errorf("failed to create locks directory: %w", err)
	}

	lockFile := filepath.Join(lockDir, fmt.Sprintf("instance_%s.lock", instanceID))

	// Check if lock file exists
	if data, err := os.ReadFile(lockFile); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			// Check if process is still running
			process, err := os.FindProcess(pid)
			if err == nil {
				// Try to send signal 0 to check if process exists
				err := process.Signal(syscall.Signal(0))
				if err == nil {
					// Process is still running
					return fmt.Errorf("instance %s is locked by process %d", instanceID, pid)
				}
			}
		}
		// Lock is stale, remove it
		os.Remove(lockFile)
	}

	// Create new lock file with current PID
	pid := os.Getpid()
	if err := os.WriteFile(lockFile, []byte(fmt.Sprintf("%d", pid)), 0600); err != nil {
		return fmt.Errorf("failed to create lock file: %w", err)
	}

	return nil
}

// ReleaseLock removes the lock file
func ReleaseLock(instanceID string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	lockFile := filepath.Join(homeDir, ".thunder", "locks", fmt.Sprintf("instance_%s.lock", instanceID))
	if err := os.Remove(lockFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

	return nil
}
