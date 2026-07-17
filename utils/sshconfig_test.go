package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSSHConfigUsesExactHostMatch(t *testing.T) {
	configPath := setupSSHConfigTest(t, `Host tnr-10
    HostName 192.0.2.10
    User ubuntu

Host tnr-1
    HostName 192.0.2.101
    User ubuntu
`)

	require.NoError(t, UpdateSSHConfig("1", "198.51.100.1", 2222, "uuid-1", nil, nil))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	config := string(data)

	assert.Contains(t, config, "Host tnr-10\n    HostName 192.0.2.10")
	assert.Equal(t, 1, strings.Count(config, "Host tnr-1\n"))
	assert.Contains(t, config, "Host tnr-1\n    HostName 198.51.100.1")
	assert.NotContains(t, config, "192.0.2.101")
}

func TestUpdateSSHConfigPreservesFollowingMatchBlock(t *testing.T) {
	configPath := setupSSHConfigTest(t, `Host tnr-1
    HostName 192.0.2.101
    User ubuntu

Match user deploy
    IdentityFile ~/.ssh/deploy
`)

	require.NoError(t, UpdateSSHConfig("1", "198.51.100.1", 22, "uuid-1", nil, nil))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	config := string(data)

	assert.Contains(t, config, "Host tnr-1\n    HostName 198.51.100.1")
	assert.NotContains(t, config, "192.0.2.101")
	assert.Contains(t, config, "Match user deploy\n    IdentityFile ~/.ssh/deploy")
}

func TestUpdateSSHConfigPreservesFollowingHostBlock(t *testing.T) {
	configPath := setupSSHConfigTest(t, `Host tnr-1
    HostName 192.0.2.101

Host personal
    HostName personal.example.com
    User developer
    Port 2222
    IdentityFile ~/.ssh/personal
    IdentitiesOnly yes
    ForwardAgent no
    StrictHostKeyChecking yes
`)

	require.NoError(t, UpdateSSHConfig("1", "198.51.100.1", 22, "uuid-1", nil, nil))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	config := string(data)

	assert.Contains(t, config, "Host tnr-1\n    HostName 198.51.100.1")
	assert.Contains(t, config, `Host personal
    HostName personal.example.com
    User developer
    Port 2222
    IdentityFile ~/.ssh/personal
    IdentitiesOnly yes
    ForwardAgent no
    StrictHostKeyChecking yes`)
}

func setupSSHConfigTest(t *testing.T, content string) string {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("TNR_HOME", filepath.Join(homeDir, ".thunder"))

	sshDir := filepath.Join(homeDir, ".ssh")
	require.NoError(t, os.MkdirAll(sshDir, 0o700))

	configPath := filepath.Join(sshDir, "config")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))
	return configPath
}
