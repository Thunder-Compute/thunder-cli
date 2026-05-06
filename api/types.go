package api

import (
	"context"

	"github.com/Thunder-Compute/thunder-cli/pkg/types"
)

type (
	Instance               = types.InstanceListItem
	InstanceMode           = types.InstanceMode
	CreateInstanceRequest  = types.InstanceCreateRequest
	CreateInstanceResponse = types.InstanceCreateResponse
	InstanceModifyRequest  = types.InstanceModifyRequest
	InstanceModifyResponse = types.InstanceModifyResponse
	AddSSHKeyResponse      = types.InstanceAddKeyResponse
	CreateSnapshotRequest  = types.CreateSnapshotRequest
	CreateSnapshotResponse = types.CreateSnapshotResponse
	Snapshot               = types.Snapshot
	ListSnapshotsResponse  = types.ListSnapshotsResponse
	ValidateTokenResult    = types.ValidateTokenResponse
)

// StorageRange defines min/max storage in GB.
type StorageRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// GpuSpecConfig represents a single GPU configuration entry.
type GpuSpecConfig struct {
	DisplayName        string       `json:"displayName"`
	VramGB             int          `json:"vramGB"`
	GpuCount           int          `json:"gpuCount"`
	Mode               string       `json:"mode"`
	VcpuOptions        []int        `json:"vcpuOptions"`
	RamPerVCPUGiB      int          `json:"ramPerVCPUGiB"`
	StorageGB          StorageRange `json:"storageGB"`
	EphemeralStorageGB StorageRange `json:"ephemeralStorageGB"`
}

// GPUAvailabilityResponse represents GPU availability returned by /v1/status.
type GPUAvailabilityResponse struct {
	GPUType map[string]map[string]string `json:"gpu_type"`
	Specs   map[string]string            `json:"specs"`
}

// TemplateEntry represents a template with its key, used for ordered iteration.
type TemplateEntry struct {
	Key      string
	Template types.EnvironmentTemplate
}

// DeleteInstanceResponse is CLI-specific (constructed by client, not from API).
type DeleteInstanceResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// ConnectClient defines the interface for API operations used by the connect command.
// This interface allows for mocking in tests.
type ConnectClient interface {
	ListInstances() ([]Instance, error)
	ListInstancesWithIPUpdateCtx(ctx context.Context) ([]Instance, error)
	AddSSHKeyCtx(ctx context.Context, instanceID string) (*AddSSHKeyResponse, error)
}
