package utils

import (
	"encoding/base64"
	"encoding/hex"
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

type HashAlgorithm string

const (
	HashAlgoUnknown HashAlgorithm = ""
	HashAlgoSHA256  HashAlgorithm = "sha256"
	HashAlgoMD5     HashAlgorithm = "md5"
)

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
	configOutput, _ := ExecuteSSHCommand(client, fmt.Sprintf("cat %s 2>/dev/null || echo '{}'", thunderConfigPath))

	var existingConfig ThunderConfig
	_ = json.Unmarshal([]byte(strings.TrimSpace(configOutput)), &existingConfig)

	configNeedsUpdate := existingConfig.DeviceID == "" || existingConfig.GPUType != gpuType || existingConfig.GPUCount != gpuCount

	expectedHash := NormalizeHash(binaryHash)
	hashAlgorithm := DetectHashAlgorithm(expectedHash)
	if hashAlgorithm == HashAlgoUnknown {
		hashAlgorithm = HashAlgoSHA256
	}

	existingHash := ""
	if expectedHash != "" {
		if h, err := GetInstanceBinaryHash(client, hashAlgorithm); err == nil {
			existingHash = h
		}
	}

	binaryNeedsUpdate := expectedHash == "" || existingHash == "" || existingHash != expectedHash

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

func NormalizeHash(hash string) string {
	trimmed := strings.TrimSpace(hash)
	if trimmed == "" {
		return ""
	}

	lowered := strings.ToLower(trimmed)
	if isHexString(lowered) {
		return lowered
	}

	if converted := decodeBase64Hash(trimmed); converted != "" {
		return converted
	}
	if converted := decodeBase64Hash(lowered); converted != "" {
		return converted
	}

	return lowered
}

func isHexString(value string) bool {
	if value == "" || len(value)%2 != 0 {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func decodeBase64Hash(value string) string {
	decoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}

	for _, enc := range decoders {
		if decoded, err := enc.DecodeString(value); err == nil {
			return hex.EncodeToString(decoded)
		}
	}

	return ""
}

func DetectHashAlgorithm(hash string) HashAlgorithm {
	switch len(hash) {
	case 64:
		if isHexString(hash) {
			return HashAlgoSHA256
		}
	case 32:
		if isHexString(hash) {
			return HashAlgoMD5
		}
	}
	return HashAlgoUnknown
}

func GetInstanceBinaryHash(client *SSHClient, algorithm HashAlgorithm) (string, error) {
	var cmd string
	switch algorithm {
	case HashAlgoMD5:
		cmd = fmt.Sprintf("md5sum %s 2>/dev/null | awk '{print $1}' || echo ''", thunderLibPath)
	default:
		cmd = fmt.Sprintf("sha256sum %s 2>/dev/null | awk '{print $1}' || echo ''", thunderLibPath)
	}

	output, err := ExecuteSSHCommand(client, cmd)
	if err != nil {
		return "", err
	}
	return NormalizeHash(output), nil
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
curl -sL %s -o /tmp/libthunder.tmp && mv /tmp/libthunder.tmp %s
sudo ln -sf %s %s
echo "%s" | sudo tee %s > /dev/null
echo "%s" | base64 -d > %s
sudo ln -sf %s /etc/thunder/config.json
echo "%s" | base64 -d > %s
sudo ln -sf %s %s
touch %s
' > /dev/null 2>&1 &`,
		thunderConfigDir,
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
