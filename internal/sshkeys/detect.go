package sshkeys

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DetectedKey represents a locally detected SSH public key.
type DetectedKey struct {
	Name      string // Filename without .pub (e.g., "id_ed25519")
	Path      string // Full path to .pub file
	PublicKey string // File contents (trimmed)
	KeyType   string // ssh-rsa, ssh-ed25519, etc.
}

var validKeyPrefixes = []string{
	"ssh-rsa ",
	"ssh-ed25519 ",
	"ssh-dss ",
	"ecdsa-sha2-nistp256 ",
	"ecdsa-sha2-nistp384 ",
	"ecdsa-sha2-nistp521 ",
	"sk-ssh-ed25519@openssh.com ",
	"sk-ecdsa-sha2-nistp256@openssh.com ",
}

// DetectLocalKeys scans ~/.ssh/ for valid SSH public keys.
func DetectLocalKeys() ([]DetectedKey, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sshDir := filepath.Join(home, ".ssh")
	matches, err := filepath.Glob(filepath.Join(sshDir, "*.pub"))
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var keys []DetectedKey

	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		content := strings.TrimSpace(string(data))
		if !isValidSSHPublicKey(content) {
			continue
		}

		if seen[content] {
			continue
		}
		seen[content] = true

		parts := strings.Fields(content)
		keyType := parts[0]

		name := strings.TrimSuffix(filepath.Base(path), ".pub")
		keys = append(keys, DetectedKey{
			Name:      name,
			Path:      path,
			PublicKey: content,
			KeyType:   keyType,
		})
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Name < keys[j].Name
	})

	return keys, nil
}

// normalizePublicKey extracts the first two space-separated fields (type + base64 data),
// stripping any trailing comment or whitespace for comparison purposes.
func normalizePublicKey(key string) string {
	parts := strings.Fields(strings.TrimSpace(key))
	if len(parts) >= 2 {
		return parts[0] + " " + parts[1]
	}
	return strings.TrimSpace(key)
}

// FindPrivateKeyForPublicKey scans ~/.ssh/*.pub for a key matching the given public key,
// then returns the path to the corresponding private key file.
func FindPrivateKeyForPublicKey(targetPublicKey string) (string, error) {
	keys, err := DetectLocalKeys()
	if err != nil {
		return "", fmt.Errorf("failed to scan local SSH keys: %w", err)
	}

	normalizedTarget := normalizePublicKey(targetPublicKey)

	for _, key := range keys {
		if normalizePublicKey(key.PublicKey) == normalizedTarget {
			privateKeyPath := strings.TrimSuffix(key.Path, ".pub")
			if _, err := os.Stat(privateKeyPath); err != nil {
				return "", fmt.Errorf("found matching public key at %s but no private key exists at %s", key.Path, privateKeyPath)
			}
			return privateKeyPath, nil
		}
	}

	return "", fmt.Errorf("no local private key found matching the saved key — ensure the corresponding private key exists in ~/.ssh/")
}

func isValidSSHPublicKey(key string) bool {
	for _, prefix := range validKeyPrefixes {
		if strings.HasPrefix(key, prefix) {
			parts := strings.Fields(key)
			if len(parts) >= 2 && len(parts[1]) > 10 {
				return true
			}
		}
	}
	return false
}
