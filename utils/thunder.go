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
	checkCmd := fmt.Sprintf(`cat %s 2>/dev/null || echo '{}'; echo '---SEPARATOR---'; sha256sum %s 2>/dev/null | awk '{print $1}' || echo ''`, thunderConfigPath, thunderLibPath)
	checkOutput, _ := ExecuteSSHCommand(client, checkCmd)

	var existingConfig ThunderConfig
	var existingHash string
	parts := strings.SplitN(checkOutput, "---SEPARATOR---", 2)
	if len(parts) >= 1 {
		_ = json.Unmarshal([]byte(strings.TrimSpace(parts[0])), &existingConfig)
	}
	if len(parts) >= 2 {
		existingHash = strings.TrimSpace(parts[1])
	}

	configNeedsUpdate := existingConfig.DeviceID == "" || existingConfig.GPUType != gpuType || existingConfig.GPUCount != gpuCount

	binaryNeedsUpdate := false
	if binaryHash != "" && existingHash != binaryHash {
		binaryNeedsUpdate = true
	} else if existingHash == "" && configNeedsUpdate {
		binaryNeedsUpdate = true
	}

	if !configNeedsUpdate && !binaryNeedsUpdate {
		return nil
	}

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
	configB64 := base64.StdEncoding.EncodeToString(configJSON)
	tokenB64 := base64.StdEncoding.EncodeToString([]byte(token))

	var scriptParts []string

	scriptParts = append(scriptParts, fmt.Sprintf("mkdir -p %s", thunderConfigDir))
	scriptParts = append(scriptParts, "sudo mkdir -p /etc/thunder")

	if binaryNeedsUpdate {
		scriptParts = append(scriptParts, fmt.Sprintf("curl -sL %s -o /tmp/libthunder.tmp && mv /tmp/libthunder.tmp %s", thunderBinaryURL, thunderLibPath))
	}

	scriptParts = append(scriptParts, fmt.Sprintf("sudo ln -sf %s %s", thunderLibPath, thunderSymlink))
	scriptParts = append(scriptParts, fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", thunderSymlink, ldPreloadPath))
	scriptParts = append(scriptParts, fmt.Sprintf("echo '%s' | base64 -d > %s", configB64, thunderConfigPath))
	scriptParts = append(scriptParts, fmt.Sprintf("sudo ln -sf %s /etc/thunder/config.json", thunderConfigPath))
	scriptParts = append(scriptParts, fmt.Sprintf("echo '%s' | base64 -d > %s", tokenB64, tokenPath))
	scriptParts = append(scriptParts, fmt.Sprintf("sudo ln -sf %s %s", tokenPath, tokenSymlink))

	setupScript := strings.Join(scriptParts, " && ")
	if _, err := ExecuteSSHCommand(client, setupScript); err != nil {
		return fmt.Errorf("failed to configure Thunder virtualization: %w", err)
	}

	return nil
}

// RemoveThunderVirtualization cleans up Thunder virtualization for production mode
func RemoveThunderVirtualization(client *SSHClient, token string) error {
	tokenB64 := base64.StdEncoding.EncodeToString([]byte(token))

	cleanupScript := fmt.Sprintf(`
sudo rm -f %s && \
rm -f %s && \
sudo rm -f %s /etc/thunder/config.json && \
mkdir -p %s && \
sudo mkdir -p /etc/thunder && \
echo '%s' | base64 -d > %s && \
sudo ln -sf %s %s
`, ldPreloadPath, thunderLibPath, thunderSymlink, thunderConfigDir, tokenB64, tokenPath, tokenPath, tokenSymlink)

	if _, err := ExecuteSSHCommand(client, cleanupScript); err != nil {
		return fmt.Errorf("failed to remove Thunder virtualization: %w", err)
	}

	return nil
}

func TriggerBackgroundSetup(client *SSHClient, instanceID, deviceID, gpuType string, gpuCount int, token string) error {
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
	configB64 := base64.StdEncoding.EncodeToString(configJSON)
	tokenB64 := base64.StdEncoding.EncodeToString([]byte(token))

	bgScript := fmt.Sprintf(`nohup bash -c '
mkdir -p %s
sudo mkdir -p /etc/thunder
if [ ! -f %s ]; then
  curl -sL %s -o /tmp/libthunder.tmp && mv /tmp/libthunder.tmp %s
fi
sudo ln -sf %s %s
echo "%s" | sudo tee %s > /dev/null
echo "%s" | base64 -d > %s
sudo ln -sf %s /etc/thunder/config.json
echo "%s" | base64 -d > %s
sudo ln -sf %s %s
touch %s
' > /dev/null 2>&1 &`,
		thunderConfigDir,
		thunderLibPath,
		thunderBinaryURL, thunderLibPath,
		thunderLibPath, thunderSymlink,
		thunderSymlink, ldPreloadPath,
		configB64, thunderConfigPath,
		thunderConfigPath,
		tokenB64, tokenPath,
		tokenPath, tokenSymlink,
		ThunderSetupMarker)

	_, err = ExecuteSSHCommand(client, bgScript)
	return err
}

func TriggerBackgroundTokenSetup(client *SSHClient, token string) error {
	tokenB64 := base64.StdEncoding.EncodeToString([]byte(token))

	bgScript := fmt.Sprintf(`nohup bash -c '
mkdir -p %s
sudo mkdir -p /etc/thunder
echo "%s" | base64 -d > %s
sudo ln -sf %s %s
touch %s
' > /dev/null 2>&1 &`,
		thunderConfigDir,
		tokenB64, tokenPath,
		tokenPath, tokenSymlink,
		ThunderSetupMarker)

	_, err := ExecuteSSHCommand(client, bgScript)
	return err
}
