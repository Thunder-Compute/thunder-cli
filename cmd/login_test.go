package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoginSuccess verifies that the saveConfig function correctly saves authentication
// credentials to the config file with proper token, refresh token, and expiration time.
func TestLoginSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	authResp := AuthResponse{
		Token:        "valid_token_12345",
		RefreshToken: "refresh_token_67890",
		ExpiresIn:    3600,
	}

	err := saveConfig(authResp)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, ".thunder", "cli_config.json")
	assert.FileExists(t, configPath)

	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config Config
	err = json.Unmarshal(configData, &config)
	require.NoError(t, err)

	assert.Equal(t, "valid_token_12345", config.Token)
	assert.Equal(t, "refresh_token_67890", config.RefreshToken)
	assert.True(t, config.ExpiresAt.After(time.Now()))
}

// TestLoadConfig verifies that the LoadConfig function correctly loads authentication
// credentials from the config file and handles missing config files appropriately.
func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	originalEnv := os.Getenv("TNR_API_TOKEN")
	defer os.Setenv("TNR_API_TOKEN", originalEnv)
	os.Unsetenv("TNR_API_TOKEN")

	thunderDir := filepath.Join(tmpDir, ".thunder")
	require.NoError(t, os.MkdirAll(thunderDir, 0700))

	configFile := filepath.Join(thunderDir, "cli_config.json")
	testConfig := map[string]interface{}{
		"token":         "test_token_12345",
		"refresh_token": "test_refresh_token",
		"expires_at":    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	configData, err := json.Marshal(testConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configFile, configData, 0600))

	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "test_token_12345", config.Token)
	assert.Equal(t, "test_refresh_token", config.RefreshToken)

	os.Remove(configFile)
	config, err = LoadConfig()
	assert.Error(t, err)
	assert.Nil(t, config)
}

// TestLoadConfigFromEnvironmentVariable verifies that the LoadConfig function
// prioritizes the TNR_API_TOKEN environment variable over the saved config file.
func TestLoadConfigFromEnvironmentVariable(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	originalEnv := os.Getenv("TNR_API_TOKEN")
	defer os.Setenv("TNR_API_TOKEN", originalEnv)

	// Set environment variable
	os.Setenv("TNR_API_TOKEN", "env_token_from_variable")

	// Also create a config file with different token
	thunderDir := filepath.Join(tmpDir, ".thunder")
	require.NoError(t, os.MkdirAll(thunderDir, 0700))
	configFile := filepath.Join(thunderDir, "cli_config.json")
	testConfig := map[string]interface{}{
		"token":         "file_token_12345",
		"refresh_token": "test_refresh_token",
	}
	configData, err := json.Marshal(testConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configFile, configData, 0600))

	// Environment variable should take precedence
	config, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "env_token_from_variable", config.Token)
	assert.Empty(t, config.RefreshToken, "env token config should not have refresh token")

	// Test with env variable only (no config file)
	os.Remove(configFile)
	config, err = LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "env_token_from_variable", config.Token)
}

// TestGenerateState verifies that the generateState function creates unique,
// properly formatted state strings for OAuth authentication.
func TestGenerateState(t *testing.T) {
	state1, err := generateState()
	require.NoError(t, err)
	assert.NotEmpty(t, state1)
	assert.Len(t, state1, 44)

	state2, err := generateState()
	require.NoError(t, err)
	assert.NotEqual(t, state1, state2)
}

// TestBuildAuthURL verifies that the buildAuthURL function correctly constructs
// OAuth authentication URLs with proper state and return URI parameters.
func TestBuildAuthURL(t *testing.T) {
	state := "test_state_123"
	returnURI := "http://127.0.0.1:8080/callback"

	url := buildAuthURL(state, returnURI)

	assert.Contains(t, url, "https://console.thundercompute.com/login/vscode")
	assert.Contains(t, url, "state=test_state_123")
	assert.Contains(t, url, "return_uri=http%3A%2F%2F127.0.0.1%3A8080%2Fcallback")
}

// TestOpenBrowser verifies that the openBrowser function doesn't panic when
// called with a valid URL, even if the actual browser opening fails.
func TestOpenBrowser(t *testing.T) {
	url := "https://example.com"
	_ = openBrowser(url)
}

// TestSaveConfig verifies that the saveConfig function correctly saves
// authentication credentials with expiration time to the config file.
func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	authResp := AuthResponse{
		Token:        "test_token",
		RefreshToken: "test_refresh",
		ExpiresIn:    7200,
	}

	err := saveConfig(authResp)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, ".thunder", "cli_config.json")
	assert.FileExists(t, configPath)

	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config Config
	err = json.Unmarshal(configData, &config)
	require.NoError(t, err)

	assert.Equal(t, "test_token", config.Token)
	assert.Equal(t, "test_refresh", config.RefreshToken)
	assert.True(t, config.ExpiresAt.After(time.Now().Add(7000*time.Second)))
}

// TestSaveConfigWithExpiration verifies that the saveConfig function correctly
// handles authentication credentials without expiration time.
func TestSaveConfigWithExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	authResp := AuthResponse{
		Token:        "test_token",
		RefreshToken: "test_refresh",
		ExpiresIn:    0,
	}

	err := saveConfig(authResp)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, ".thunder", "cli_config.json")
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config Config
	err = json.Unmarshal(configData, &config)
	require.NoError(t, err)

	assert.Equal(t, "test_token", config.Token)
	assert.Equal(t, "test_refresh", config.RefreshToken)
	assert.True(t, config.ExpiresAt.IsZero())
}

// TestSaveConfigCreatesDirectory verifies that the saveConfig function creates
// the necessary .thunder directory structure when it doesn't exist.
func TestSaveConfigCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	authResp := AuthResponse{
		Token: "test_token",
	}

	err := saveConfig(authResp)
	require.NoError(t, err)

	thunderDir := filepath.Join(tmpDir, ".thunder")
	assert.DirExists(t, thunderDir)

	configFile := filepath.Join(thunderDir, "cli_config.json")
	assert.FileExists(t, configFile)
}

// TestSaveConfigPermissionError verifies that the saveConfig function properly
// handles errors when the .thunder directory cannot be created.
func TestSaveConfigPermissionError(t *testing.T) {
	tmpDir := t.TempDir()

	thunderDir := filepath.Join(tmpDir, ".thunder")
	err := os.WriteFile(thunderDir, []byte("not a directory"), 0600)
	require.NoError(t, err)

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	authResp := AuthResponse{
		Token: "test_token",
	}

	err = saveConfig(authResp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

// TestLoginCommandTableDriven provides comprehensive test coverage for various
// authentication scenarios including successful logins with and without expiration.
func TestLoginCommandTableDriven(t *testing.T) {
	tests := []struct {
		name          string
		authResp      AuthResponse
		expectError   bool
		errorContains string
	}{
		{
			name: "successful login with expiration",
			authResp: AuthResponse{
				Token:        "valid_token",
				RefreshToken: "refresh_token",
				ExpiresIn:    3600,
			},
			expectError: false,
		},
		{
			name: "successful login without expiration",
			authResp: AuthResponse{
				Token:        "valid_token",
				RefreshToken: "refresh_token",
				ExpiresIn:    0,
			},
			expectError: false,
		},
		{
			name: "login with empty token",
			authResp: AuthResponse{
				Token:        "",
				RefreshToken: "refresh_token",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			originalHome := os.Getenv("HOME")
			defer os.Setenv("HOME", originalHome)
			os.Setenv("HOME", tmpDir)

			err := saveConfig(tt.authResp)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)

				configPath := filepath.Join(tmpDir, ".thunder", "cli_config.json")
				configData, err := os.ReadFile(configPath)
				require.NoError(t, err)

				var config Config
				err = json.Unmarshal(configData, &config)
				require.NoError(t, err)

				assert.Equal(t, tt.authResp.Token, config.Token)
				assert.Equal(t, tt.authResp.RefreshToken, config.RefreshToken)
			}
		})
	}
}
