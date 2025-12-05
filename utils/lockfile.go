package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type InstanceLock struct {
	lockPath string
}

func GetLockDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".thunder", "locks")
}

func GetInstanceLockPath(instanceID string) string {
	return filepath.Join(GetLockDir(), fmt.Sprintf("tnr-%s.lock", instanceID))
}

// AcquireInstanceLock attempts to acquire an exclusive lock for the given instance.
// Returns an InstanceLock if successful, or an error if the lock cannot be acquired.
// Uses O_EXCL for atomicity; stale locks (>30 min or dead process) are automatically cleaned up.
func AcquireInstanceLock(instanceID string) (*InstanceLock, error) {
	lockDir := GetLockDir()
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create locks directory: %w", err)
	}

	lockPath := GetInstanceLockPath(instanceID)

	if data, err := os.ReadFile(lockPath); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) >= 2 {
			if pid, err := strconv.Atoi(lines[0]); err == nil {
				if timestamp, err := strconv.ParseInt(lines[1], 10, 64); err == nil {
					lockAge := time.Since(time.Unix(timestamp, 0))
					if isProcessAlive(pid) && lockAge < 30*time.Minute {
						return nil, fmt.Errorf("another 'tnr connect' session is already in progress for this instance")
					}
				}
			}
		}
		_ = os.Remove(lockPath) // Stale lock
	}

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("another 'tnr connect' session is already in progress for this instance")
		}
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}

	_, _ = file.WriteString(fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Unix()))
	file.Close()

	return &InstanceLock{lockPath: lockPath}, nil
}

// Release releases the instance lock and removes the lock file.
func (l *InstanceLock) Release() error {
	if l == nil || l.lockPath == "" {
		return nil
	}
	if err := os.Remove(l.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}
	return nil
}

// IsInstanceLocked checks if the given instance is currently locked by another process.
func IsInstanceLocked(instanceID string) bool {
	lockPath := GetInstanceLockPath(instanceID)

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return false
	}

	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		return false
	}

	timestamp, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil {
		return false
	}

	if isProcessAlive(pid) {
		lockAge := time.Since(time.Unix(timestamp, 0))
		return lockAge < 30*time.Minute
	}
	return false
}

// isProcessAlive is a best-effort check if a process is still running.
// On Linux, checks /proc/<pid>. On other platforms, relies on timestamp staleness.
func isProcessAlive(pid int) bool {
	if _, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
		return true
	}

	// On macOS/Windows, FindProcess always succeeds for any PID.
	// We rely on timestamp-based staleness detection as a fallback.
	_, err := os.FindProcess(pid)
	return err == nil
}
