package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetKeyFile returns the path to the SSH key file for a given instance UUID.
// Falls back to empty string only if ThunderDir resolution fails, which
// callers should treat as "no cached key".
func GetKeyFile(uuid string) string {
	base, err := ThunderDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "keys", uuid)
}

// KeyExists checks if the SSH key file exists for a given UUID
func KeyExists(uuid string) bool {
	keyFile := GetKeyFile(uuid)
	if keyFile == "" {
		return false
	}
	_, err := os.Stat(keyFile)
	return err == nil
}

// SavePrivateKey writes the private key to disk with appropriate permissions
func SavePrivateKey(uuid, privateKey string) error {
	keyDir, err := ThunderSubdir("keys")
	if err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	keyFile := filepath.Join(keyDir, uuid)
	if err := os.WriteFile(keyFile, []byte(privateKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	return nil
}
