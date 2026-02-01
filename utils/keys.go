package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetKeyFile returns the path to the SSH key file for a given instance UUID
func GetKeyFile(uuid string) string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".thunder", "keys", uuid)
}

// KeyExists checks if the SSH key file exists for a given UUID
func KeyExists(uuid string) bool {
	keyFile := GetKeyFile(uuid)
	_, err := os.Stat(keyFile)
	return err == nil
}

// SavePrivateKey writes the private key to disk with appropriate permissions
func SavePrivateKey(uuid, privateKey string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	keyDir := filepath.Join(homeDir, ".thunder", "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	keyFile := filepath.Join(keyDir, uuid)
	if err := os.WriteFile(keyFile, []byte(privateKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	return nil
}
