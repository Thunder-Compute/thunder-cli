package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpdateSSHConfig updates the ~/.ssh/config file with the instance connection details
func UpdateSSHConfig(instanceID, ip string, port int, keyFile string, tunnelPorts []int, templatePorts []int) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	configPath := filepath.Join(sshDir, "config")

	// Read existing config
	var existingLines []string
	if data, err := os.ReadFile(configPath); err == nil {
		existingLines = strings.Split(string(data), "\n")
	}

	// Check if entry exists
	hostName := fmt.Sprintf("tnr-%s", instanceID)
	existingIndex := -1
	inBlock := false
	blockStart := -1
	blockEnd := -1

	for i, line := range existingLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Host ") {
			if inBlock {
				blockEnd = i - 1
				break
			}
			if strings.Contains(trimmed, hostName) {
				existingIndex = i
				blockStart = i
				inBlock = true
			}
		}
	}

	if inBlock && blockEnd == -1 {
		blockEnd = len(existingLines) - 1
	}

	// Build new SSH config entry
	var configLines []string
	configLines = append(configLines, fmt.Sprintf("Host %s", hostName))
	configLines = append(configLines, fmt.Sprintf("    HostName %s", ip))
	configLines = append(configLines, "    User ubuntu")
	configLines = append(configLines, fmt.Sprintf("    IdentityFile \"%s\"", keyFile))
	configLines = append(configLines, "    IdentitiesOnly yes")
	configLines = append(configLines, "    StrictHostKeyChecking no")
	configLines = append(configLines, fmt.Sprintf("    Port %d", port))

	// Add port forwarding
	allPorts := make(map[int]bool)
	for _, p := range tunnelPorts {
		allPorts[p] = true
	}
	for _, p := range templatePorts {
		allPorts[p] = true
	}

	for port := range allPorts {
		configLines = append(configLines, fmt.Sprintf("    LocalForward %d localhost:%d", port, port))
	}

	// Update or append config
	var newLines []string
	if existingIndex != -1 {
		// Replace existing block
		newLines = append(existingLines[:blockStart], configLines...)
		if blockEnd+1 < len(existingLines) {
			newLines = append(newLines, existingLines[blockEnd+1:]...)
		}
	} else {
		// Append new entry
		newLines = existingLines
		if len(newLines) > 0 && newLines[len(newLines)-1] != "" {
			newLines = append(newLines, "")
		}
		newLines = append(newLines, configLines...)
	}

	// Write config
	content := strings.Join(newLines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write SSH config: %w", err)
	}

	return nil
}

// GetTemplateOpenPorts returns the list of open ports for a given template
func GetTemplateOpenPorts(templateName string) []int {
	// Common template port mappings
	templates := map[string][]int{
		"jupyter":     {8888},
		"vscode":      {8080},
		"rstudio":     {8787},
		"tensorboard": {6006},
		"mlflow":      {5000},
	}

	if ports, ok := templates[strings.ToLower(templateName)]; ok {
		return ports
	}

	return []int{}
}
