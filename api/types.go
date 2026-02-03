package api

import (
	"context"

	"github.com/Thunder-Compute/thundernetes/services/pkg/thundertypes"
)

type (
	Instance               = thundertypes.InstanceListItem
	Template               = thundertypes.Template
	TemplateDefaultSpecs   = thundertypes.TemplateDefaultSpecs
	InstanceMode           = thundertypes.InstanceMode
	CreateInstanceRequest  = thundertypes.InstanceCreateRequest
	CreateInstanceResponse = thundertypes.InstanceCreateResponse
	InstanceModifyRequest  = thundertypes.InstanceModifyRequest
	InstanceModifyResponse = thundertypes.InstanceModifyResponse
	AddSSHKeyResponse      = thundertypes.InstanceAddKeyResponse
	CreateSnapshotRequest  = thundertypes.CreateSnapshotRequest
	CreateSnapshotResponse = thundertypes.CreateSnapshotResponse
	Snapshot               = thundertypes.Snapshot
	ListSnapshotsResponse  = thundertypes.ListSnapshotsResponse
)

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
