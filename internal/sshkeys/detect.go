package sshkeys

import (
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
