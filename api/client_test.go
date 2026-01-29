package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		CPUCores:   8,
		GPUType:    "a6000",
		Template:   "ubuntu-22.04",
		NumGPUs:    1,
		DiskSizeGB: 100,
		Mode:       "prototyping",
	}

	jsonData, err := json.Marshal(req)
	require.NoError(t, err)

	var unmarshaled CreateInstanceRequest
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, req.CPUCores, unmarshaled.CPUCores)
	assert.Equal(t, req.GPUType, unmarshaled.GPUType)
	assert.Equal(t, req.Template, unmarshaled.Template)
	assert.Equal(t, req.NumGPUs, unmarshaled.NumGPUs)
	assert.Equal(t, req.DiskSizeGB, unmarshaled.DiskSizeGB)
	assert.Equal(t, req.Mode, unmarshaled.Mode)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(jsonData, &raw))
	assert.Equal(t, float64(req.CPUCores), raw["cpu_cores"])
	assert.Equal(t, req.GPUType, raw["gpu_type"])
	assert.Equal(t, req.Template, raw["template"])
	assert.Equal(t, float64(req.NumGPUs), raw["num_gpus"])
	assert.Equal(t, float64(req.DiskSizeGB), raw["disk_size_gb"])
	assert.Equal(t, req.Mode, raw["mode"])
}

func TestCreateInstanceResponse(t *testing.T) {
	resp := CreateInstanceResponse{
		UUID:    "test-uuid-12345",
		Message: "Instance created successfully",
	}

	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled CreateInstanceResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, resp.UUID, unmarshaled.UUID)
	assert.Equal(t, resp.Message, unmarshaled.Message)
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
	instance := Instance{
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

	jsonData, err := json.Marshal(instance)
	require.NoError(t, err)

	var unmarshaled Instance
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, instance.UUID, unmarshaled.UUID)
	assert.Equal(t, instance.Name, unmarshaled.Name)
	assert.Equal(t, instance.Status, unmarshaled.Status)
	assert.Equal(t, instance.IP, unmarshaled.IP)
	assert.Equal(t, instance.CPUCores, unmarshaled.CPUCores)
	assert.Equal(t, instance.Memory, unmarshaled.Memory)
	assert.Equal(t, instance.Storage, unmarshaled.Storage)
	assert.Equal(t, instance.GPUType, unmarshaled.GPUType)
	assert.Equal(t, instance.NumGPUs, unmarshaled.NumGPUs)
	assert.Equal(t, instance.Mode, unmarshaled.Mode)
	assert.Equal(t, instance.Template, unmarshaled.Template)
	assert.Equal(t, instance.CreatedAt, unmarshaled.CreatedAt)
	assert.Equal(t, instance.Port, unmarshaled.Port)
	assert.Equal(t, instance.K8s, unmarshaled.K8s)
	assert.Equal(t, instance.Promoted, unmarshaled.Promoted)
}

func TestTemplateStruct(t *testing.T) {
	template := Template{
		Key:                 "ubuntu-22.04",
		DisplayName:         "Ubuntu 22.04",
		ExtendedDescription: "Ubuntu 22.04 LTS with development tools",
		AutomountFolders:    []string{"/workspace", "/data"},
		CleanupCommands:     []string{"sudo apt update", "sudo apt upgrade"},
		OpenPorts:           []int{8080, 3000, 22},
		StartupCommands:     []string{"sudo systemctl start docker"},
		StartupMinutes:      5,
		Version:             1,
		DefaultSpecs: ThunderTemplateDefaultSpecs{
			Cores:   8,
			GpuType: "a6000",
			NumGpus: 1,
			Storage: 100,
		},
	}

	jsonData, err := json.Marshal(template)
	require.NoError(t, err)

	var unmarshaled Template
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
	testKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7..."
	resp := AddSSHKeyResponse{
		UUID:    "test-uuid-12345",
		Key:     &testKey,
		Success: true,
		Message: "Key added successfully",
	}

	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled AddSSHKeyResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, resp.UUID, unmarshaled.UUID)
	assert.Equal(t, *resp.Key, *unmarshaled.Key)
	assert.Equal(t, resp.Success, unmarshaled.Success)
	assert.Equal(t, resp.Message, unmarshaled.Message)
}

func TestDeviceIDResponse(t *testing.T) {
	resp := DeviceIDResponse{
		ID: "device-12345",
	}

	jsonData, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled DeviceIDResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, resp.ID, unmarshaled.ID)
}

func TestNewClientWithCustomURL(t *testing.T) {
	token := "test_token_12345"
	customURL := "https://staging-api.thundercompute.com:8443"
	client := NewClient(token, customURL)

	assert.NotNil(t, client)
	assert.Equal(t, customURL, client.baseURL)
	assert.Equal(t, token, client.token)
}
