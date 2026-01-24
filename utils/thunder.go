package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

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
	thunderBinaryURL  = "https://storage.googleapis.com/client-binary/client_linux_x86_64"
	thunderConfigDir  = "/home/ubuntu/.thunder"
	thunderConfigPath = "/home/ubuntu/.thunder/config.json"
	thunderLibPath    = "/home/ubuntu/.thunder/libthunder.so"
	thunderSymlink    = "/etc/thunder/libthunder.so"
	ldPreloadPath     = "/etc/ld.so.preload"
	tokenPath         = "/home/ubuntu/.thunder/token"
	tokenSymlink      = "/etc/thunder/token"
)

func GetThunderConfig(client *SSHClient) (*ThunderConfig, error) {
	// Use stdout-only to avoid stderr pollution from ld.so.preload errors
	output, err := ExecuteSSHCommandStdoutOnly(client, fmt.Sprintf("cat %s 2>/dev/null || echo '{}'", thunderConfigPath))
	if err != nil {
		return nil, err
	}

	output = filterLdSoErrors(output)

	var config ThunderConfig
	if err := json.Unmarshal([]byte(output), &config); err != nil {
		return nil, fmt.Errorf("failed to parse Thunder config: %w", err)
	}

	return &config, nil
}

// filterLdSoErrors prevents stderr pollution from breaking output parsing when /etc/ld.so.preload references a missing binary
func filterLdSoErrors(output string) string {
	lines := strings.Split(output, "\n")
	var filtered []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		isLdSoError := strings.Contains(line, "ld.so: object") ||
			strings.Contains(line, "cannot be preloaded") ||
			strings.Contains(line, "ignored") ||
			strings.HasPrefix(line, "error: ld.so:")
		if !isLdSoError {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

// CleanupLdSoPreloadIfBinaryMissing prevents stderr pollution from breaking command output parsing
func CleanupLdSoPreloadIfBinaryMissing(client *SSHClient) error {
	checkCmd := fmt.Sprintf("test -f %s && echo 'exists' || echo 'missing'", thunderLibPath)
	output, err := ExecuteSSHCommandStdoutOnly(client, checkCmd)
	if err != nil {
		return err
	}
	output = filterLdSoErrors(output)

	if strings.TrimSpace(output) == "missing" {
		cleanupCmd := fmt.Sprintf("sudo sed -i '/%s/d' %s 2>/dev/null || sudo rm -f %s 2>/dev/null || true", thunderSymlink, ldPreloadPath, ldPreloadPath)
		_, err := ExecuteSSHCommand(client, cleanupCmd)
		return err
	}
	return nil
}

func ConfigureThunderVirtualization(client *SSHClient, instanceID, deviceID, gpuType string, gpuCount int, token, binaryHash string, existingConfig *ThunderConfig) error {
	expectedHash := NormalizeHash(binaryHash)
	isValidHash := expectedHash != "" && len(expectedHash) == 32 && IsHexString(expectedHash)
	hashAlgorithm := DetectHashAlgorithm(expectedHash)
	existingHash := ""
	if isValidHash {
		if h, err := GetInstanceBinaryHash(client, hashAlgorithm); err == nil {
			existingHash = h
		}
	}

	// If binary hash matches, no update needed
	if isValidHash && existingHash != "" && existingHash == expectedHash {
		return nil
	}

	binaryNeedsUpdate := !isValidHash || existingHash == "" || existingHash != expectedHash

	tokenB64 := base64.StdEncoding.EncodeToString([]byte(token))

	var scriptParts []string
	scriptParts = append(scriptParts, fmt.Sprintf("mkdir -p %s", thunderConfigDir))
	scriptParts = append(scriptParts, "sudo mkdir -p /etc/thunder")

	if binaryNeedsUpdate {
		scriptParts = append(scriptParts, fmt.Sprintf("curl -sL %s -o /tmp/libthunder.tmp && mv /tmp/libthunder.tmp %s", thunderBinaryURL, thunderLibPath))
		scriptParts = append(scriptParts, fmt.Sprintf("sudo ln -sf %s %s", thunderLibPath, thunderSymlink))
		scriptParts = append(scriptParts, fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", thunderSymlink, ldPreloadPath))
	}

	// Always ensure token is set (in case it changed)
	scriptParts = append(scriptParts, fmt.Sprintf("echo '%s' | base64 -d > %s", tokenB64, tokenPath))
	scriptParts = append(scriptParts, fmt.Sprintf("sudo ln -sf %s %s", tokenPath, tokenSymlink))

	if len(scriptParts) > 0 {
		setupScript := strings.Join(scriptParts, " && ")
		if _, err := ExecuteSSHCommand(client, setupScript); err != nil {
			return fmt.Errorf("failed to configure Thunder virtualization: %w", err)
		}
	}

	return nil
}

func NormalizeHash(hash string) string {
	trimmed := strings.TrimSpace(hash)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

func IsHexString(value string) bool {
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

func DetectHashAlgorithm(hash string) HashAlgorithm {
	if hash == "" {
		return HashAlgoUnknown
	}
	switch len(hash) {
	case 64:
		if IsHexString(hash) {
			return HashAlgoSHA256
		}
	case 32:
		if IsHexString(hash) {
			return HashAlgoMD5
		}
	}
	return HashAlgoMD5
}

func GetInstanceBinaryHash(client *SSHClient, algorithm HashAlgorithm) (string, error) {
	var cmd string
	switch algorithm {
	case HashAlgoMD5:
		cmd = fmt.Sprintf("md5sum %s 2>/dev/null | awk '{print $1}' || echo ''", thunderLibPath)
	default:
		cmd = fmt.Sprintf("sha256sum %s 2>/dev/null | awk '{print $1}' || echo ''", thunderLibPath)
	}

	output, err := ExecuteSSHCommandStdoutOnly(client, cmd)
	if err != nil {
		return "", err
	}

	output = filterLdSoErrors(output)
	normalized := NormalizeHash(output)
	return normalized, nil
}

// RemoveThunderVirtualization production: removes binary/config, keeps token
func RemoveThunderVirtualization(client *SSHClient, token string) error {
	productionCommands := []string{
		fmt.Sprintf("sudo rm -f %s || true", ldPreloadPath),
		"sudo touch /etc/ld.so.preload || true",
		"sudo chown root:root /etc/ld.so.preload || true",
		"sudo chmod 644 /etc/ld.so.preload || true",
		fmt.Sprintf("sudo rm -f %s || true", thunderLibPath),
		fmt.Sprintf("sudo rm -f %s || true", thunderConfigPath),
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
' > /dev/null 2>&1 &`,
		thunderConfigDir,
		thunderBinaryURL, thunderLibPath,
		thunderLibPath, thunderSymlink,
		thunderSymlink, ldPreloadPath,
		configB64, thunderConfigPath,
		thunderConfigPath,
		tokenB64, tokenPath,
		tokenPath, tokenSymlink)

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
' > /dev/null 2>&1 &`,
		thunderConfigDir,
		tokenB64, tokenPath,
		tokenPath, tokenSymlink)

	_, err := ExecuteSSHCommand(client, bgScript)
	return err
}
