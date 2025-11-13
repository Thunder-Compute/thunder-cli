package cmd

import (
	"os"
	"path/filepath"
	"strings"
)

func getCurrentBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func isPMManaged(binPath string) bool {
	return strings.Contains(binPath, "/opt/homebrew/") ||
		strings.Contains(binPath, "/usr/local/Cellar/") ||
		strings.Contains(binPath, "\\scoop\\apps\\") ||
		strings.Contains(binPath, "WindowsApps")
}


