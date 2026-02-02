package api

import (
	"context"

	"github.com/Thunder-Compute/thundernetes/services/pkg/apitypes"
)

type (
	Instance               = apitypes.InstanceListItem
	Template               = apitypes.Template
	TemplateDefaultSpecs   = apitypes.TemplateDefaultSpecs
	InstanceMode           = apitypes.InstanceMode
	CreateInstanceRequest  = apitypes.InstanceCreateRequest
	CreateInstanceResponse = apitypes.InstanceCreateResponse
	InstanceModifyRequest  = apitypes.InstanceModifyRequest
	InstanceModifyResponse = apitypes.InstanceModifyResponse
	AddSSHKeyResponse      = apitypes.InstanceAddKeyResponse
	CreateSnapshotRequest  = apitypes.CreateSnapshotRequest
	CreateSnapshotResponse = apitypes.CreateSnapshotResponse
	Snapshot               = apitypes.Snapshot
	ListSnapshotsResponse  = apitypes.ListSnapshotsResponse
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
