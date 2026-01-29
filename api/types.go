package api

import (
	"context"
	"time"
)

type Instance struct {
	ID               string    `json:"-"`
	UUID             string    `json:"uuid"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	IP               string    `json:"ip"`
	CPUCores         string    `json:"cpuCores"`
	Memory           string    `json:"memory"`
	Storage          int       `json:"storage"`
	GPUType          string    `json:"gpuType"`
	NumGPUs          string    `json:"numGpus"`
	Mode             string    `json:"mode"`
	Template         string    `json:"template"`
	CreatedAt        string    `json:"createdAt"`
	Port             int       `json:"port"`
	HttpPorts        []int     `json:"httpPorts,omitempty"`
	K8s              bool      `json:"k8s"`
	Promoted         bool      `json:"promoted"`
	ProvisioningTime time.Time `json:"provisioningTime,omitempty"`
}

type ThunderTemplateDefaultSpecs struct {
	Cores   int    `json:"cores"`
	GpuType string `json:"gpu_type"`
	NumGpus int    `json:"num_gpus"`
	Storage int    `json:"storage"`
}

type Template struct {
	Key                 string                      `json:"-"`
	DisplayName         string                      `json:"displayName"`
	ExtendedDescription string                      `json:"extendedDescription,omitempty"`
	AutomountFolders    []string                    `json:"automountFolders"`
	CleanupCommands     []string                    `json:"cleanupCommands"`
	OpenPorts           []int                       `json:"openPorts"`
	StartupCommands     []string                    `json:"startupCommands"`
	StartupMinutes      int                         `json:"startupMinutes,omitempty"`
	Version             int                         `json:"version,omitempty"`
	DefaultSpecs        ThunderTemplateDefaultSpecs `json:"defaultSpecs"`
}

type CreateInstanceRequest struct {
	CPUCores   int    `json:"cpu_cores"`
	GPUType    string `json:"gpu_type"`
	Template   string `json:"template"`
	NumGPUs    int    `json:"num_gpus"`
	DiskSizeGB int    `json:"disk_size_gb"`
	Mode       string `json:"mode"`
}

type CreateInstanceResponse struct {
	UUID       string `json:"uuid"`
	Message    string `json:"message"`
	Identifier int    `json:"identifier"`
	Key        string `json:"key"`
}

type DeleteInstanceResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

type InstanceModifyRequest struct {
	CpuCores    *int    `json:"cpu_cores,omitempty"`
	GpuType     *string `json:"gpu_type,omitempty"`
	NumGpus     *int    `json:"num_gpus,omitempty"`
	DiskSizeGb  *int    `json:"disk_size_gb,omitempty"`
	Mode        *string `json:"mode,omitempty"`
	AddPorts    []int   `json:"add_ports,omitempty"`
	RemovePorts []int   `json:"remove_ports,omitempty"`
}

type InstanceModifyResponse struct {
	Identifier   string  `json:"identifier"`
	InstanceName string  `json:"instance_name"`
	Mode         *string `json:"mode,omitempty"`
	GpuType      *string `json:"gpu_type,omitempty"`
	NumGpus      *int    `json:"num_gpus,omitempty"`
	Message      string  `json:"message,omitempty"`
	HttpPorts    []int   `json:"http_ports,omitempty"`
}

// AddSSHKeyRequest represents the request to add an SSH key
type AddSSHKeyRequest struct {
	PublicKey *string `json:"public_key,omitempty"`
}

// AddSSHKeyResponse represents the response from adding an SSH key
type AddSSHKeyResponse struct {
	UUID    string  `json:"uuid"`
	Key     *string `json:"key,omitempty"`
	Success bool    `json:"success"`
	Message string  `json:"message,omitempty"`
}

type DeviceIDResponse struct {
	ID string `json:"id"`
}

// CreateSnapshotRequest represents the request to create a snapshot
type CreateSnapshotRequest struct {
	InstanceId string `json:"instanceId"`
	Name       string `json:"name"`
}

// CreateSnapshotResponse represents the response from creating a snapshot
type CreateSnapshotResponse struct {
	Message string `json:"message"`
}

// Snapshot represents a snapshot of an instance
type Snapshot struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	MinimumDiskSizeGB int    `json:"minimumDiskSizeGb"`
	Status            string `json:"status"`
	CreatedAt         int64  `json:"createdAt"`
}

// ListSnapshotsResponse represents the list of snapshots
type ListSnapshotsResponse []Snapshot

// ConnectClient defines the interface for API operations used by the connect command.
// This interface allows for mocking in tests.
type ConnectClient interface {
	ListInstances() ([]Instance, error)
	ListInstancesWithIPUpdateCtx(ctx context.Context) ([]Instance, error)
	AddSSHKeyCtx(ctx context.Context, instanceID string, req *AddSSHKeyRequest) (*AddSSHKeyResponse, error)
}
