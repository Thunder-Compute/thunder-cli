package cmd

import (
	"strings"
	"testing"

	"github.com/joshuawatkins04/thunder-cli-draft/api"
	"github.com/joshuawatkins04/thunder-cli-draft/internal/testutils"
	"github.com/stretchr/testify/assert"
)

// TestStatusCommand verifies that the status command is properly initialized
// with the correct usage and description.
func TestStatusCommand(t *testing.T) {
	env := testutils.SetupTestEnvironment(t)
	defer env.Cleanup()

	assert.NotNil(t, statusCmd)
	assert.Equal(t, "status", statusCmd.Use)
	assert.Equal(t, "List and monitor Thunder Compute instances", statusCmd.Short)
}

// TestStatusCommandFlags verifies that the status command has the expected
// command-line flags properly defined.
func TestStatusCommandFlags(t *testing.T) {
	assert.NotNil(t, statusCmd.Flags().Lookup("no-wait"))
}

// TestInstanceStatus verifies that instance status values are correctly
// assigned and retrieved for various status types.
func TestInstanceStatus(t *testing.T) {
	statuses := []string{"RUNNING", "STOPPED", "STARTING", "DELETING", "ERROR"}

	for _, status := range statuses {
		instance := &api.Instance{
			ID:     "test-instance",
			Status: status,
		}
		assert.Equal(t, status, instance.Status)
	}
}

// TestInstanceFields verifies that all instance fields are correctly
// assigned and can be retrieved with their expected values.
func TestInstanceFields(t *testing.T) {
	instance := &api.Instance{
		ID:        "test-instance",
		UUID:      "uuid-123",
		Name:      "Test Instance",
		Status:    "RUNNING",
		IP:        "192.168.1.100",
		CPUCores:  "8",
		Memory:    "32GB",
		Storage:   100,
		GPUType:   "T4",
		NumGPUs:   "1",
		Mode:      "prototyping",
		Template:  "ubuntu-22.04",
		CreatedAt: "2023-10-01T10:00:00Z",
		Port:      22,
		K8s:       false,
		Promoted:  false,
	}

	assert.Equal(t, "test-instance", instance.ID)
	assert.Equal(t, "uuid-123", instance.UUID)
	assert.Equal(t, "Test Instance", instance.Name)
	assert.Equal(t, "RUNNING", instance.Status)
	assert.Equal(t, "192.168.1.100", instance.IP)
	assert.Equal(t, "8", instance.CPUCores)
	assert.Equal(t, "32GB", instance.Memory)
	assert.Equal(t, 100, instance.Storage)
	assert.Equal(t, "T4", instance.GPUType)
	assert.Equal(t, "1", instance.NumGPUs)
	assert.Equal(t, "prototyping", instance.Mode)
	assert.Equal(t, "ubuntu-22.04", instance.Template)
	assert.Equal(t, "2023-10-01T10:00:00Z", instance.CreatedAt)
	assert.Equal(t, 22, instance.Port)
	assert.False(t, instance.K8s)
	assert.False(t, instance.Promoted)
}

// func TestStatusCommandValidation(t *testing.T) {
// 	t.Skip("Skipping status command validation test - Args function issues")
// }

// TestNoWaitFlag verifies that the noWait flag has the correct default value.
func TestNoWaitFlag(t *testing.T) {
	assert.False(t, noWait)
}

// TestRunStatus verifies that the runStatus function properly handles
// authentication errors when no valid configuration is available.
func TestRunStatus(t *testing.T) {
	err := runStatus()
	assert.Error(t, err)
	errStr := err.Error()
	isAuthError := strings.Contains(errStr, "not authenticated") || strings.Contains(errStr, "authentication")
	isTTYError := strings.Contains(errStr, "TTY") || strings.Contains(errStr, "/dev/tty") || strings.Contains(errStr, "error running status TUI")
	assert.True(t, isAuthError || isTTYError, "Expected auth or TTY error, got: %v", err)
}
