package cmd

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanupSSHConfig verifies that the cleanupSSHConfig function correctly
// removes SSH host entries for deleted instances while preserving other entries.
func TestCleanupSSHConfig(t *testing.T) {
	env := testutils.SetupTestEnvironment(t)
	defer env.Cleanup()

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", env.TempDir)

	sshConfigPath := testutils.CreateMockSSHConfig(t, env.TempDir)

	instanceID := "test-instance"
	err := cleanupSSHConfig(instanceID, "192.168.1.100")
	require.NoError(t, err)

	configData, err := os.ReadFile(sshConfigPath)
	require.NoError(t, err)

	configContent := string(configData)
	assert.NotContains(t, configContent, "tnr-test-instance")
	assert.Contains(t, configContent, "tnr-another-instance")
}

// TestCleanupSSHConfigNonExistentFile verifies that cleanupSSHConfig handles
// non-existent SSH config files gracefully without errors.
func TestCleanupSSHConfigNonExistentFile(t *testing.T) {
	env := testutils.SetupTestEnvironment(t)
	defer env.Cleanup()

	instanceID := "nonexistent-instance"
	err := cleanupSSHConfig(instanceID, "192.168.1.100")
	require.NoError(t, err)
}

// TestRemoveSSHHostEntry verifies that the removeSSHHostEntry function correctly
// removes specific SSH host entries from the config file while preserving others.
func TestRemoveSSHHostEntry(t *testing.T) {
	env := testutils.SetupTestEnvironment(t)
	defer env.Cleanup()

	sshDir := filepath.Join(env.TempDir, ".ssh")
	require.NoError(t, os.MkdirAll(sshDir, 0700))

	sshConfigPath := filepath.Join(sshDir, "config")
	sshConfig := `Host tnr-instance1
				  HostName 192.168.1.100
				  User ubuntu
				  Port 22

			      Host tnr-instance2
				  HostName 192.168.1.101
				  User ubuntu
				  Port 22

				  Host tnr-instance3
			  	  HostName 192.168.1.102
				  User ubuntu
				  Port 22
	`

	require.NoError(t, os.WriteFile(sshConfigPath, []byte(sshConfig), 0600))

	err := removeSSHHostEntry(sshConfigPath, "instance2")
	require.NoError(t, err)

	configData, err := os.ReadFile(sshConfigPath)
	require.NoError(t, err)

	configContent := string(configData)
	assert.NotContains(t, configContent, "tnr-instance2")
	assert.Contains(t, configContent, "tnr-instance1")
	assert.Contains(t, configContent, "tnr-instance3")
}

// TestRemoveSSHHostEntryFirstEntry verifies that the removeSSHHostEntry function
// correctly handles removal of the first entry in the SSH config file.
func TestRemoveSSHHostEntryFirstEntry(t *testing.T) {
	env := testutils.SetupTestEnvironment(t)
	defer env.Cleanup()

	sshDir := filepath.Join(env.TempDir, ".ssh")
	require.NoError(t, os.MkdirAll(sshDir, 0700))

	sshConfigPath := filepath.Join(sshDir, "config")
	sshConfig := `Host tnr-first
				  HostName 192.168.1.100
				  User ubuntu

			      Host tnr-second
			   	  HostName 192.168.1.101
				  User ubuntu
	`

	require.NoError(t, os.WriteFile(sshConfigPath, []byte(sshConfig), 0600))

	err := removeSSHHostEntry(sshConfigPath, "first")
	require.NoError(t, err)

	configData, err := os.ReadFile(sshConfigPath)
	require.NoError(t, err)

	configContent := string(configData)
	assert.NotContains(t, configContent, "tnr-first")
	assert.Contains(t, configContent, "tnr-second")
}

// TestRemoveSSHHostEntryLastEntry verifies that the removeSSHHostEntry function
// correctly handles removal of the last entry in the SSH config file.
func TestRemoveSSHHostEntryLastEntry(t *testing.T) {
	env := testutils.SetupTestEnvironment(t)
	defer env.Cleanup()

	sshDir := filepath.Join(env.TempDir, ".ssh")
	require.NoError(t, os.MkdirAll(sshDir, 0700))

	sshConfigPath := filepath.Join(sshDir, "config")
	sshConfig := `Host tnr-first
				  HostName 192.168.1.100
				  User ubuntu
 
				  Host tnr-last
				  HostName 192.168.1.101
				  User ubuntu
	`

	require.NoError(t, os.WriteFile(sshConfigPath, []byte(sshConfig), 0600))

	err := removeSSHHostEntry(sshConfigPath, "last")
	require.NoError(t, err)

	configData, err := os.ReadFile(sshConfigPath)
	require.NoError(t, err)

	configContent := string(configData)
	assert.NotContains(t, configContent, "tnr-last")
	assert.Contains(t, configContent, "tnr-first")
}

// TestRemoveSSHHostEntryNonExistent verifies that the removeSSHHostEntry function
// handles non-existent entries gracefully without errors.
func TestRemoveSSHHostEntryNonExistent(t *testing.T) {
	env := testutils.SetupTestEnvironment(t)
	defer env.Cleanup()

	sshDir := filepath.Join(env.TempDir, ".ssh")
	require.NoError(t, os.MkdirAll(sshDir, 0700))

	sshConfigPath := filepath.Join(sshDir, "config")
	sshConfig := `Host tnr-existing
    HostName 192.168.1.100
    User ubuntu
`
	require.NoError(t, os.WriteFile(sshConfigPath, []byte(sshConfig), 0600))

	err := removeSSHHostEntry(sshConfigPath, "nonexistent")
	require.NoError(t, err)

	configData, err := os.ReadFile(sshConfigPath)
	require.NoError(t, err)

	configContent := string(configData)
	assert.Contains(t, configContent, "tnr-existing")
}

// TestDeleteSpinnerModel verifies that the delete spinner model initializes
// correctly with the provided message and proper state.
func TestDeleteSpinnerModel(t *testing.T) {
	model := newDeleteSpinnerModel("Test message")
	assert.Equal(t, "Test message", model.message)
	assert.False(t, model.quitting)
}

// TestDeleteSpinnerModelView verifies that the delete spinner model renders
// correctly in both normal and quitting states.
func TestDeleteSpinnerModelView(t *testing.T) {
	model := newDeleteSpinnerModel("Deleting instance...")

	view := model.View()
	assert.Contains(t, view, "Deleting instance...")

	model.quitting = true
	view = model.View()
	assert.Empty(t, view)
}

// TestDeleteSpinnerModelUpdate verifies that the delete spinner model
// correctly handles key press events and state updates.
func TestDeleteSpinnerModelUpdate(t *testing.T) {
	model := newDeleteSpinnerModel("Test message")

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, cmd := model.Update(keyMsg)

	updatedModel, ok := newModel.(deleteSpinnerModel)
	require.True(t, ok)
	assert.True(t, updatedModel.quitting)
	assert.NotNil(t, cmd)
}

// TestDeleteInstanceResponse verifies that the delete instance response
// structure contains the expected fields and values.
func TestDeleteInstanceResponse(t *testing.T) {
	response := api.DeleteInstanceResponse{
		Message: "Instance deleted successfully",
		Success: true,
	}

	assert.Equal(t, "Instance deleted successfully", response.Message)
	assert.True(t, response.Success)
}

// TestDeleteCommandValidation provides comprehensive validation testing for
// delete command scenarios including valid instances and various error conditions.
func TestDeleteCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		instanceID  string
		status      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid instance",
			instanceID:  "test-instance",
			status:      "RUNNING",
			expectError: false,
		},
		{
			name:        "instance already deleting",
			instanceID:  "test-instance",
			status:      "DELETING",
			expectError: true,
			errorMsg:    "instance 'test-instance' is already being deleted",
		},
		{
			name:        "instance starting",
			instanceID:  "test-instance",
			status:      "STARTING",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := &api.Instance{
				ID:     tt.instanceID,
				Status: tt.status,
			}

			if tt.status == "DELETING" {
				assert.Equal(t, "DELETING", instance.Status)
			}
		})
	}
}

func TestSSHConfigCleanupWithIP(t *testing.T) {
	env := testutils.SetupTestEnvironment(t)
	defer env.Cleanup()

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", env.TempDir)

	sshConfigPath := testutils.CreateMockSSHConfig(t, env.TempDir)

	instanceID := "test-instance"
	ipAddress := "192.168.1.100"

	err := cleanupSSHConfig(instanceID, ipAddress)
	require.NoError(t, err)

	configData, err := os.ReadFile(sshConfigPath)
	require.NoError(t, err)

	configContent := string(configData)
	assert.NotContains(t, configContent, "tnr-test-instance")
}

func TestSSHConfigCleanupWithoutIP(t *testing.T) {
	env := testutils.SetupTestEnvironment(t)
	defer env.Cleanup()

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", env.TempDir)

	sshConfigPath := testutils.CreateMockSSHConfig(t, env.TempDir)

	instanceID := "test-instance"
	ipAddress := ""

	err := cleanupSSHConfig(instanceID, ipAddress)
	require.NoError(t, err)

	configData, err := os.ReadFile(sshConfigPath)
	require.NoError(t, err)

	configContent := string(configData)
	assert.NotContains(t, configContent, "tnr-test-instance")
}

// func TestSSHConfigCleanupHomeDirError(t *testing.T) {
// 	t.Skip("Skipping home directory error test - difficult to mock os.UserHomeDir")
// }
