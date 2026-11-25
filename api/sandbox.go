package api

import (
	"context"
	"fmt"
	"net/http"
)

type StartSandboxRequest struct {
	Name            string `json:"name,omitempty"`
	ImageID         string `json:"image_id"`
	WorkspaceID     string `json:"workspace_id,omitempty"`
	TimingSensitive bool   `json:"timing_sensitive,omitempty"`
	Spec            struct {
		CPUCount   int    `json:"cpu_count"`
		MemoryGiB  int    `json:"memory_gib"`
		StorageGiB int    `json:"storage_gib"`
		GPUType    string `json:"gpu_type"`
		GPUCount   int    `json:"gpu_count"`
	} `json:"spec"`
	Lifetime struct {
		EnforceTTL    bool `json:"enforce_ttl"`
		MaxTTLSeconds int  `json:"max_ttl_seconds"`
	} `json:"lifetime"`
	NetworkPolicy struct {
		InternetAccess string `json:"internet_access"`
	} `json:"network_policy"`
}

type StartSandboxResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

func (c *Client) StartSandbox(ctx context.Context, request StartSandboxRequest) (StartSandboxResponse, error) {
	var response StartSandboxResponse
	if err := c.doRequest(ctx, http.MethodPost, "/v1/sandboxes/start", request, &response); err != nil {
		return response, err
	}
	if response.ID == "" {
		return response, fmt.Errorf("Thunder accepted the start request without returning a sandbox ID. Check the Thunder console before retrying")
	}
	return response, nil
}
