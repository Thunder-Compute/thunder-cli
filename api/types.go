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
	SSHKey                 = types.SSHKey
	SSHKeyAddRequest       = types.SSHKeyAddRequest
	SSHKeyAddResponse      = types.SSHKeyAddResponse
	SSHKeyListResponse     = types.SSHKeyListResponse
	SSHKeyDeleteResponse   = types.SSHKeyDeleteResponse
)

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
	AddSSHKeyToInstanceWithPublicKey(instanceID, publicKey string) (*AddSSHKeyResponse, error)
}
