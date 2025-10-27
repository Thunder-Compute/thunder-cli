package testutils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type TestEnvironment struct {
	TempDir    string
	ThunderDir string
	CredFile   string
	ConfigFile string
	BinaryFile string
	Cleanup    func()
}

func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	tmpDir := t.TempDir()
	thunderDir := filepath.Join(tmpDir, ".thunder")

	if err := os.MkdirAll(thunderDir, 0700); err != nil {
		t.Fatalf("Failed to create thunder directory: %v", err)
	}

	credFile := filepath.Join(thunderDir, "token")
	if err := os.WriteFile(credFile, []byte("test_token_12345"), 0600); err != nil {
		t.Fatalf("Failed to create credential file: %v", err)
	}

	configFile := filepath.Join(thunderDir, "config.json")
	config := map[string]interface{}{
		"token":         "test_token_12345",
		"refresh_token": "test_refresh_token",
		"expires_at":    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configFile, configData, 0600); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	binaryFile := filepath.Join(thunderDir, "libthunder.so")
	if err := os.WriteFile(binaryFile, []byte("mock binary content"), 0700); err != nil {
		t.Fatalf("Failed to create binary file: %v", err)
	}

	cleanup := func() {
	}

	return &TestEnvironment{
		TempDir:    tmpDir,
		ThunderDir: thunderDir,
		CredFile:   credFile,
		ConfigFile: configFile,
		BinaryFile: binaryFile,
		Cleanup:    cleanup,
	}
}

func CreateMockSSHConfig(t *testing.T, tempDir string) string {
	sshDir := filepath.Join(tempDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("Failed to create SSH directory: %v", err)
	}

	sshConfigPath := filepath.Join(sshDir, "config")
	sshConfig := `Host tnr-test-instance
				  HostName 192.168.1.100
				  User ubuntu
				  Port 22
				  IdentityFile ~/.ssh/id_rsa

				  Host tnr-another-instance
				  HostName 192.168.1.101
				  User ubuntu
				  Port 22
				  IdentityFile ~/.ssh/id_rsa
	`

	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0600); err != nil {
		t.Fatalf("Failed to create SSH config: %v", err)
	}

	return sshConfigPath
}

func CreateMockKnownHosts(t *testing.T, tempDir string) string {
	sshDir := filepath.Join(tempDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("Failed to create SSH directory: %v", err)
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	knownHosts := `192.168.1.100 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7...
				   192.168.1.101 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC8...
`
	if err := os.WriteFile(knownHostsPath, []byte(knownHosts), 0600); err != nil {
		t.Fatalf("Failed to create known_hosts: %v", err)
	}

	return knownHostsPath
}
