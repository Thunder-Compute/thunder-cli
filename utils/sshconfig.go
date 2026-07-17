package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpdateSSHConfig updates the ~/.ssh/config file with the instance connection details
func UpdateSSHConfig(instanceID, ip string, port int, uuid string, tunnelPorts []int, templatePorts []int) error {
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

	hostName := fmt.Sprintf("tnr-%s", instanceID)

	// Build new SSH config entry
	keyFile := GetKeyFile(uuid)
	var configLines []string
	configLines = append(configLines, fmt.Sprintf("Host %s", hostName))
	configLines = append(configLines, fmt.Sprintf("    HostName %s", ip))
	configLines = append(configLines, "    User ubuntu")
	configLines = append(configLines, fmt.Sprintf("    IdentityFile \"%s\"", keyFile))
	configLines = append(configLines, "    IdentitiesOnly yes")
	configLines = append(configLines, "    IdentityAgent none")
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

	newLines := upsertSSHHostBlock(existingLines, configLines, hostName)

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

func upsertSSHHostBlock(existingLines, configLines []string, hostName string) []string {
	blockStart := -1
	blockEnd := len(existingLines)

	for i, line := range existingLines {
		if blockStart == -1 {
			if isExactSSHHostDirective(line, hostName) {
				blockStart = i
			}
			continue
		}

		if isSSHConfigBlockStart(line) {
			blockEnd = i
			break
		}
	}

	if blockStart != -1 {
		newLines := make([]string, 0, len(existingLines)-(blockEnd-blockStart)+len(configLines))
		newLines = append(newLines, existingLines[:blockStart]...)
		newLines = append(newLines, configLines...)
		newLines = append(newLines, existingLines[blockEnd:]...)
		return newLines
	}

	newLines := make([]string, 0, len(existingLines)+len(configLines)+1)
	newLines = append(newLines, existingLines...)
	if len(newLines) > 0 && newLines[len(newLines)-1] != "" {
		newLines = append(newLines, "")
	}
	return append(newLines, configLines...)
}

func isExactSSHHostDirective(line, hostName string) bool {
	fields := strings.Fields(line)
	return len(fields) == 2 && strings.EqualFold(fields[0], "Host") && fields[1] == hostName
}

func isSSHConfigBlockStart(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && (strings.EqualFold(fields[0], "Host") || strings.EqualFold(fields[0], "Match"))
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
