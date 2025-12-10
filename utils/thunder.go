package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// ThunderConfig represents the Thunder virtualization configuration
type ThunderConfig struct {
	InstanceID string `json:"instanceId"`
	DeviceID   string `json:"deviceId"`
	GPUType    string `json:"gpuType"`
	GPUCount   int    `json:"gpuCount"`
}

const (
	thunderBinaryURL   = "https://storage.googleapis.com/client-binary/client_linux_x86_64"
	thunderConfigDir   = "/home/ubuntu/.thunder"
	thunderConfigPath  = "/home/ubuntu/.thunder/config.json"
	thunderLibPath     = "/home/ubuntu/.thunder/libthunder.so"
	ThunderSetupMarker = "/home/ubuntu/.thunder/setup_complete"
	thunderSymlink     = "/etc/thunder/libthunder.so"
	ldPreloadPath      = "/etc/ld.so.preload"
	tokenPath          = "/home/ubuntu/.thunder/token"
	tokenSymlink       = "/etc/thunder/token"
)

func IsInstanceSetupComplete(client *SSHClient) bool {
	output, err := ExecuteSSHCommand(client, fmt.Sprintf("test -f %s && echo yes || echo no", ThunderSetupMarker))
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) == "yes"
}

func MarkInstanceSetupComplete(client *SSHClient) error {
	_, err := ExecuteSSHCommand(client, fmt.Sprintf("mkdir -p %s && touch %s", thunderConfigDir, ThunderSetupMarker))
	return err
}

// GetThunderConfig reads the Thunder configuration from the instance
func GetThunderConfig(client *SSHClient) (*ThunderConfig, error) {
	output, err := ExecuteSSHCommand(client, fmt.Sprintf("cat %s 2>/dev/null || echo '{}'", thunderConfigPath))
	if err != nil {
		return nil, err
	}

	var config ThunderConfig
	if err := json.Unmarshal([]byte(output), &config); err != nil {
		return nil, fmt.Errorf("failed to parse Thunder config: %w", err)
	}

	return &config, nil
}

// ConfigureThunderVirtualization sets up GPU virtualization for prototyping mode
func ConfigureThunderVirtualization(client *SSHClient, instanceID, deviceID, gpuType string, gpuCount int, token, binaryHash string) error {
	// Check if update is needed
	existingConfig, _ := GetThunderConfig(client)
	existingHash, _ := ExecuteSSHCommand(client, fmt.Sprintf("sha256sum %s 2>/dev/null | awk '{print $1}' || echo ''", thunderLibPath))
	existingHash = strings.TrimSpace(existingHash)

	needsUpdate := false
	if existingConfig.DeviceID == "" || existingConfig.GPUType != gpuType || existingConfig.GPUCount != gpuCount {
		needsUpdate = true
	}
	if existingHash != binaryHash {
		needsUpdate = true
	}

	if !needsUpdate {
		return nil
	}

	// Create Thunder directory
	if _, err := ExecuteSSHCommand(client, fmt.Sprintf("mkdir -p %s", thunderConfigDir)); err != nil {
		return fmt.Errorf("failed to create Thunder directory: %w", err)
	}

	// Download Thunder binary
	downloadCmd := fmt.Sprintf("curl -L %s -o /tmp/libthunder.tmp && mv /tmp/libthunder.tmp %s", thunderBinaryURL, thunderLibPath)
	if _, err := ExecuteSSHCommand(client, downloadCmd); err != nil {
		return fmt.Errorf("failed to download Thunder binary: %w", err)
	}

	// Create symlink
	symlinkCmd := fmt.Sprintf("sudo mkdir -p /etc/thunder && sudo ln -sf %s %s", thunderLibPath, thunderSymlink)
	if _, err := ExecuteSSHCommand(client, symlinkCmd); err != nil {
		return fmt.Errorf("failed to create Thunder symlink: %w", err)
	}

	// Setup LD_PRELOAD
	ldPreloadCmd := fmt.Sprintf("echo '%s' | sudo tee %s", thunderSymlink, ldPreloadPath)
	if _, err := ExecuteSSHCommand(client, ldPreloadCmd); err != nil {
		return fmt.Errorf("failed to setup LD_PRELOAD: %w", err)
	}

	// Write configuration
	config := ThunderConfig{
		InstanceID: instanceID,
		DeviceID:   deviceID,
		GPUType:    gpuType,
		GPUCount:   gpuCount,
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// base64-encode so it contains no quotes
	configB64 := base64.StdEncoding.EncodeToString(configJSON)
	writeConfigCmd := fmt.Sprintf("echo '%s' | base64 -d > %s && sudo ln -sf %s /etc/thunder/config.json", configB64, thunderConfigPath, thunderConfigPath)
	if _, err := ExecuteSSHCommand(client, writeConfigCmd); err != nil {
		return fmt.Errorf("failed to write Thunder config: %w", err)
	}

	// Write token
	tokenB64 := base64.StdEncoding.EncodeToString([]byte(token))
	writeTokenCmd := fmt.Sprintf("echo '%s' | base64 -d > %s && sudo ln -sf %s %s", tokenB64, tokenPath, tokenPath, tokenSymlink)
	if _, err := ExecuteSSHCommand(client, writeTokenCmd); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}

	return nil
}

// RemoveThunderVirtualization cleans up Thunder virtualization for production mode
func RemoveThunderVirtualization(client *SSHClient, token string) error {
	// Clear LD_PRELOAD
	if _, err := ExecuteSSHCommand(client, fmt.Sprintf("sudo rm -f %s", ldPreloadPath)); err != nil {
		return fmt.Errorf("failed to clear LD_PRELOAD: %w", err)
	}

	// Remove Thunder binary
	if _, err := ExecuteSSHCommand(client, fmt.Sprintf("rm -f %s", thunderLibPath)); err != nil {
		return fmt.Errorf("failed to remove Thunder binary: %w", err)
	}

	// Remove symlinks
	if _, err := ExecuteSSHCommand(client, fmt.Sprintf("sudo rm -f %s /etc/thunder/config.json", thunderSymlink)); err != nil {
		return fmt.Errorf("failed to remove Thunder symlinks: %w", err)
	}

	// Write token (keep this for API access)
	tokenB64 := base64.StdEncoding.EncodeToString([]byte(token))
	writeTokenCmd := fmt.Sprintf("mkdir -p %s && echo '%s' | base64 -d > %s && sudo mkdir -p /etc/thunder && sudo ln -sf %s %s", thunderConfigDir, tokenB64, tokenPath, tokenPath, tokenSymlink)
	if _, err := ExecuteSSHCommand(client, writeTokenCmd); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}

	return nil
}
