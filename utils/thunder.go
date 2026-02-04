package utils

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	thunderConfigDir = "/home/ubuntu/.thunder"
	thunderLibPath   = "/home/ubuntu/.thunder/libthunder.so"
	thunderSymlink   = "/etc/thunder/libthunder.so"
	ldPreloadPath    = "/etc/ld.so.preload"
	tokenPath        = "/home/ubuntu/.thunder/token"
)

// SetupToken sets up the authentication token on the instance
func SetupToken(client *SSHClient, token string) error {
	tokenB64 := base64.StdEncoding.EncodeToString([]byte(token))

	tokenCommands := []string{
		fmt.Sprintf("mkdir -p %s", thunderConfigDir),
		fmt.Sprintf("echo '%s' | base64 -d > %s", tokenB64, tokenPath),
		fmt.Sprintf("chmod 600 %s", tokenPath),
		"sudo sed -i '/export TNR_API_TOKEN/d' /home/ubuntu/.bashrc || true",
		"echo 'export TNR_API_TOKEN=\"$(cat /home/ubuntu/.thunder/token)\"' | sudo tee -a /home/ubuntu/.bashrc > /dev/null || true",
	}

	setupScript := strings.Join(tokenCommands, " && ")

	if _, err := ExecuteSSHCommand(client, setupScript); err != nil {
		return fmt.Errorf("failed to set up token: %w", err)
	}

	return nil
}

// RemoveThunderVirtualization removes Thunder binary/config for production mode and sets up token
func RemoveThunderVirtualization(client *SSHClient, token string) error {
	productionCommands := []string{
		fmt.Sprintf("sudo rm -f %s || true", ldPreloadPath),
		"sudo touch /etc/ld.so.preload || true",
		"sudo chown root:root /etc/ld.so.preload || true",
		"sudo chmod 644 /etc/ld.so.preload || true",
		fmt.Sprintf("sudo rm -f %s || true", thunderLibPath),
		"sudo rm -rf /etc/thunder || true",
		fmt.Sprintf("echo '%s' | base64 -d > /tmp/token.tmp", base64.StdEncoding.EncodeToString([]byte(token))),
		"sudo install -d -m 755 /home/ubuntu/.thunder || true",
		"sudo install -m 600 -o ubuntu -g ubuntu /tmp/token.tmp /home/ubuntu/.thunder/token || true",
		"rm -f /tmp/token.tmp || true",
		"sudo sed -i '/export TNR_API_TOKEN/d' /home/ubuntu/.bashrc || true",
		"echo 'export TNR_API_TOKEN=\"$(cat /home/ubuntu/.thunder/token)\"' | sudo tee -a /home/ubuntu/.bashrc > /dev/null || true",
	}

	cleanupScript := strings.Join(productionCommands, " ; ")

	if _, err := ExecuteSSHCommand(client, cleanupScript); err != nil {
		return fmt.Errorf("failed to remove Thunder virtualization: %w", err)
	}

	return nil
}
