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

func detectPackageManager(binPath string) string {
	p := strings.ToLower(binPath)
	if strings.Contains(p, "/opt/homebrew/") || strings.Contains(p, "/usr/local/cellar/") {
		return "homebrew"
	}
	if strings.Contains(p, "\\scoop\\apps\\") {
		return "scoop"
	}
	if strings.Contains(p, "windowsapps") {
		return "winget"
	}
	return ""
}
