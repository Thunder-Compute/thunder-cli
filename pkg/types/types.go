// Package types provides API request/response types for the Thunder Compute CLI.
// These types are copied from the monorepo to allow the CLI to be built standalone.
package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// InstanceStatus represents the status of a compute instance.
type InstanceStatus string

const (
	InstanceStatus_Provisioning InstanceStatus = "PROVISIONING"
	InstanceStatus_Queued       InstanceStatus = "QUEUED"
	InstanceStatus_Starting     InstanceStatus = "STARTING"
	InstanceStatus_Running      InstanceStatus = "RUNNING"
	InstanceStatus_Stopping     InstanceStatus = "STOPPING"
	InstanceStatus_Stopped      InstanceStatus = "STOPPED"
	InstanceStatus_Pending      InstanceStatus = "PENDING"
	InstanceStatus_Unknown      InstanceStatus = "UNKNOWN"
	InstanceStatus_Modifying    InstanceStatus = "MODIFYING"
	InstanceStatus_Snapshotting InstanceStatus = "SNAPPING"
	InstanceStatus_Restoring    InstanceStatus = "RESTORING"
)

// InstanceMode represents the mode of operation for an instance.
type InstanceMode string

const (
	InstanceMode_Prototyping InstanceMode = "prototyping"
	InstanceMode_Production  InstanceMode = "production"
)

// IsValidInstanceMode checks if the given mode is a valid InstanceMode.
func IsValidInstanceMode(mode InstanceMode) bool {
	switch mode {
	case InstanceMode_Prototyping, InstanceMode_Production:
		return true
	default:
		return false
	}
}

// InstanceListResponse is the API response for listing instances.
type InstanceListResponse map[string]InstanceListItem

// InstanceListItem represents a single instance in the list response.
type InstanceListItem struct {
	ID               string    `json:"id,omitempty"`
	IP               *string   `json:"ip,omitempty"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	CreatedAt        string    `json:"createdAt"`
	UUID             string    `json:"uuid"`
	Storage          int       `json:"storage"`
	CPUCores         string    `json:"cpuCores"`
	Template         string    `json:"template"`
	GPUType          string    `json:"gpuType"`
	NumGPUs          string    `json:"numGpus"`
	Memory           string    `json:"memory"`
	Promoted         bool      `json:"promoted"`
	Mode             string    `json:"mode"`
	Port             int       `json:"port"`
	HTTPPorts        []int     `json:"httpPorts,omitempty"`
	K8s              bool      `json:"k8s"`
	ProvisioningTime time.Time `json:"provisioningTime,omitempty"`
	RestoringTime    time.Time `json:"restoringTime,omitempty"`
	SnapshotSize     int64     `json:"snapshotSize,omitempty"`
	SSHPublicKeys    []string  `json:"sshPublicKeys,omitempty"`
}

// GetIP returns the IP address or empty string if nil.
func (i InstanceListItem) GetIP() string {
	if i.IP == nil {
		return ""
	}
	return *i.IP
}

// UnmarshalJSON implements custom JSON unmarshaling for InstanceListItem
// to handle CPUCores field that can be either int or string.
func (i *InstanceListItem) UnmarshalJSON(data []byte) error {
	// Define a temporary struct with CPUCores as interface{} to handle both types
	type Alias InstanceListItem
	temp := struct {
		*Alias
		CPUCores interface{} `json:"cpuCores"`
	}{
		Alias: (*Alias)(i),
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Handle CPUCores field conversion
	switch v := temp.CPUCores.(type) {
	case string:
		i.CPUCores = v
	case int:
		i.CPUCores = fmt.Sprintf("%d", v)
	case float64:
		i.CPUCores = fmt.Sprintf("%.0f", v)
	case nil:
		i.CPUCores = ""
	default:
		return fmt.Errorf("unexpected type for cpuCores field: %T", v)
	}

	return nil
}

// InstanceCreateRequest represents the request body for creating an instance.
type InstanceCreateRequest struct {
	CPUCores   int          `json:"cpu_cores"`
	Mode       InstanceMode `json:"mode"`
	Template   string       `json:"template"`
	GPUType    string       `json:"gpu_type"`
	NumGPUs    int          `json:"num_gpus"`
	DiskSizeGB int          `json:"disk_size_gb"`
	PublicKey  string       `json:"public_key,omitempty"`
}

// InstanceCreateResponse represents the response from creating an instance.
type InstanceCreateResponse struct {
	UUID       string `json:"uuid"`
	Key        string `json:"key"`
	Identifier int    `json:"identifier"`
}

// InstanceAddKeyResponse represents the response from adding an SSH key.
type InstanceAddKeyResponse struct {
	UUID    string  `json:"uuid"`
	Key     *string `json:"key,omitempty"`
	Success bool    `json:"success"`
	Message string  `json:"message,omitempty"`
}

// InstanceModifyRequest represents the request body for modifying an instance.
type InstanceModifyRequest struct {
	CPUCores    *int          `json:"cpu_cores,omitempty"`
	GPUType     *string       `json:"gpu_type,omitempty"`
	NumGPUs     *int          `json:"num_gpus,omitempty"`
	DiskSizeGB  *int          `json:"disk_size_gb,omitempty"`
	Mode        *InstanceMode `json:"mode,omitempty"`
	AddPorts    []int         `json:"add_ports,omitempty"`
	RemovePorts []int         `json:"remove_ports,omitempty"`
}

// InstanceStopResponse represents the response from stopping an instance.
type InstanceStopResponse struct {
	Message string `json:"message"`
}

// InstanceStartResponse represents the response from starting an instance.
type InstanceStartResponse struct {
	Message string `json:"message"`
}

// InstanceModifyResponse represents the response from modifying an instance.
type InstanceModifyResponse struct {
	Identifier   string  `json:"identifier"`
	InstanceName string  `json:"instance_name"`
	Mode         *string `json:"mode,omitempty"`
	GPUType      *string `json:"gpu_type,omitempty"`
	NumGPUs      *int    `json:"num_gpus,omitempty"`
	HTTPPorts    []int   `json:"http_ports,omitempty"`
}

// CreateSnapshotRequest represents the request to create a snapshot.
type CreateSnapshotRequest struct {
	InstanceID string `json:"instanceId"`
	Name       string `json:"name"`
}

// CreateSnapshotResponse represents the response from creating a snapshot.
type CreateSnapshotResponse struct {
	Message string `json:"message"`
}

// Snapshot represents a user snapshot.
type Snapshot struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	MinimumDiskSizeGB int    `json:"minimumDiskSizeGb"`
	Status            string `json:"status"`
	CreatedAt         int64  `json:"createdAt"`
}

// ListSnapshotsResponse is the list of user snapshots.
type ListSnapshotsResponse []Snapshot

// TemplateDefaultSpecs represents the default specs for a thunder template.
type TemplateDefaultSpecs struct {
	Cores    *int    `json:"cores,omitempty"`
	GPUType  *string `json:"gpu_type,omitempty"`
	NumGPUs  *int    `json:"num_gpus,omitempty"`
	Storage  *int    `json:"storage,omitempty"`
	Template *string `json:"template,omitempty"`
}

// EnvironmentTemplate represents a thunder template for instance creation.
type EnvironmentTemplate struct {
	DisplayName         string                `json:"displayName"`
	ExtendedDescription string                `json:"extendedDescription,omitempty"`
	AutomountFolders    []string              `json:"automountFolders"`
	CleanupCommands     []string              `json:"cleanupCommands"`
	OpenPorts           []int                 `json:"openPorts"`
	StartupCommands     []string              `json:"startupCommands"`
	StartupMinutes      *int                  `json:"startupMinutes,omitempty"`
	Version             *int                  `json:"version,omitempty"`
	DefaultSpecs        *TemplateDefaultSpecs `json:"defaultSpecs,omitempty"`
	Default             *bool                 `json:"default,omitempty"`
}

// ThunderTemplatesResponse is the response from the /thunder-templates endpoint.
type ThunderTemplatesResponse map[string]EnvironmentTemplate

// SSHKey represents an organization-level SSH public key.
type SSHKey struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	KeyType     string `json:"key_type"`
	CreatedAt   int64  `json:"created_at"`
}

// SSHKeyAddRequest is the request body for adding an SSH key.
type SSHKeyAddRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

// SSHKeyAddResponse is the response from adding an SSH key.
type SSHKeyAddResponse struct {
	Key     SSHKey `json:"key"`
	Message string `json:"message"`
}

// SSHKeyListResponse is the list of organization SSH keys.
type SSHKeyListResponse []SSHKey

// SSHKeyDeleteResponse is the response from deleting an SSH key.
type SSHKeyDeleteResponse struct {
	Message string `json:"message"`
}
