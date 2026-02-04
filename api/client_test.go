package api

import (
	"encoding/json"
	"testing"

	"github.com/Thunder-Compute/thundernetes/services/pkg/thundertypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestNewClient(t *testing.T) {
	token := "test_token_12345"
	baseURL := "https://api.thundercompute.com:8443"
	client := NewClient(token, baseURL)

	assert.NotNil(t, client)
	assert.Equal(t, baseURL, client.baseURL)
	assert.Equal(t, token, client.token)
}

func TestCreateInstanceRequest(t *testing.T) {
	req := CreateInstanceRequest{
		CpuCores:   8,
		GpuType:    "a6000",
		Template:   "ubuntu-22.04",
		NumGpus:    1,
		DiskSizeGb: 100,
		Mode:       InstanceMode("prototyping"),
	}

	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	var unmarshaled CreateInstanceRequest
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, req.CpuCores, unmarshaled.CpuCores)
	assert.Equal(t, req.GpuType, unmarshaled.GpuType)
	assert.Equal(t, req.Template, unmarshaled.Template)
	assert.Equal(t, req.NumGpus, unmarshaled.NumGpus)
	assert.Equal(t, req.DiskSizeGb, unmarshaled.DiskSizeGb)
	assert.Equal(t, req.Mode, unmarshaled.Mode)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(jsonData, &raw))
	assert.Equal(t, float64(req.CpuCores), raw["cpu_cores"])
	assert.Equal(t, req.GpuType, raw["gpu_type"])
	assert.Equal(t, req.Template, raw["template"])
	assert.Equal(t, float64(req.NumGpus), raw["num_gpus"])
	assert.Equal(t, float64(req.DiskSizeGb), raw["disk_size_gb"])
	assert.Equal(t, string(req.Mode), raw["mode"])
}

func TestCreateInstanceResponse(t *testing.T) {
	resp := CreateInstanceResponse{
		Uuid:       "test-uuid-12345",
		Key:        "test-key",
		Identifier: 1,
	}

	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled CreateInstanceResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, resp.Uuid, unmarshaled.Uuid)
	assert.Equal(t, resp.Key, unmarshaled.Key)
	assert.Equal(t, resp.Identifier, unmarshaled.Identifier)
}

func TestDeleteInstanceResponse(t *testing.T) {
	resp := DeleteInstanceResponse{
		Message: "Instance deleted successfully",
		Success: true,
	}

	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled DeleteInstanceResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, resp.Message, unmarshaled.Message)
	assert.Equal(t, resp.Success, unmarshaled.Success)
}

func TestInstanceStruct(t *testing.T) {
	ip := "192.168.1.100"
	instance := Instance{
		ID:        "test-instance",
		Uuid:      "uuid-123",
		Name:      "Test Instance",
		Status:    "RUNNING",
		Ip:        &ip,
		CpuCores:  "8",
		Memory:    "32GB",
		Storage:   100,
		GpuType:   "T4",
		NumGpus:   "1",
		Mode:      "prototyping",
		Template:  "ubuntu-22.04",
		CreatedAt: "2023-10-01T10:00:00Z",
		Port:      22,
		K8s:       false,
		Promoted:  false,
	}

	jsonData, err := json.Marshal(instance)
	require.NoError(t, err)

	var unmarshaled Instance
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, instance.Uuid, unmarshaled.Uuid)
	assert.Equal(t, instance.Name, unmarshaled.Name)
	assert.Equal(t, instance.Status, unmarshaled.Status)
	assert.Equal(t, instance.GetIP(), unmarshaled.GetIP())
	assert.Equal(t, instance.CpuCores, unmarshaled.CpuCores)
	assert.Equal(t, instance.Memory, unmarshaled.Memory)
	assert.Equal(t, instance.Storage, unmarshaled.Storage)
	assert.Equal(t, instance.GpuType, unmarshaled.GpuType)
	assert.Equal(t, instance.NumGpus, unmarshaled.NumGpus)
	assert.Equal(t, instance.Mode, unmarshaled.Mode)
	assert.Equal(t, instance.Template, unmarshaled.Template)
	assert.Equal(t, instance.CreatedAt, unmarshaled.CreatedAt)
	assert.Equal(t, instance.Port, unmarshaled.Port)
	assert.Equal(t, instance.K8s, unmarshaled.K8s)
	assert.Equal(t, instance.Promoted, unmarshaled.Promoted)
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func TestTemplateStruct(t *testing.T) {
	template := thundertypes.EnvironmentTemplate{
		DisplayName:         "Ubuntu 22.04",
		ExtendedDescription: "Ubuntu 22.04 LTS with development tools",
		AutomountFolders:    []string{"/workspace", "/data"},
		CleanupCommands:     []string{"sudo apt update", "sudo apt upgrade"},
		OpenPorts:           []int{8080, 3000, 22},
		StartupCommands:     []string{"sudo systemctl start docker"},
		StartupMinutes:      intPtr(5),
		Version:             intPtr(1),
		DefaultSpecs: &thundertypes.TemplateDefaultSpecs{
			Cores:   intPtr(8),
			GpuType: strPtr("a6000"),
			NumGpus: intPtr(1),
			Storage: intPtr(100),
		},
	}

	jsonData, err := json.Marshal(template)
	require.NoError(t, err)

	var unmarshaled thundertypes.EnvironmentTemplate
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, template.DisplayName, unmarshaled.DisplayName)
	assert.Equal(t, template.ExtendedDescription, unmarshaled.ExtendedDescription)
	assert.Equal(t, template.AutomountFolders, unmarshaled.AutomountFolders)
	assert.Equal(t, template.CleanupCommands, unmarshaled.CleanupCommands)
	assert.Equal(t, template.OpenPorts, unmarshaled.OpenPorts)
	assert.Equal(t, template.StartupCommands, unmarshaled.StartupCommands)
	assert.Equal(t, template.StartupMinutes, unmarshaled.StartupMinutes)
	assert.Equal(t, template.Version, unmarshaled.Version)
	assert.Equal(t, template.DefaultSpecs, unmarshaled.DefaultSpecs)
}

func TestAddSSHKeyResponse(t *testing.T) {
	resp := AddSSHKeyResponse{
		Uuid: "test-uuid-12345",
		Key:  ptr("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7..."),
	}

	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled AddSSHKeyResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, resp.Uuid, unmarshaled.Uuid)
	assert.Equal(t, resp.Key, unmarshaled.Key)
}

func TestNewClientWithCustomURL(t *testing.T) {
	token := "test_token_12345"
	customURL := "https://staging-api.thundercompute.com:8443"
	client := NewClient(token, customURL)

	assert.NotNil(t, client)
	assert.Equal(t, customURL, client.baseURL)
	assert.Equal(t, token, client.token)
}
