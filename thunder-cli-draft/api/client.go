package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL = "https://api.thundercompute.com:8443"
)

type Instance struct {
	ID        string `json:"-"`
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	IP        string `json:"ip"`
	CPUCores  string `json:"cpuCores"`
	Memory    string `json:"memory"`
	Storage   int    `json:"storage"`
	GPUType   string `json:"gpuType"`
	NumGPUs   string `json:"numGpus"`
	Mode      string `json:"mode"`
	Template  string `json:"template"`
	CreatedAt string `json:"createdAt"`
	Port      int    `json:"port"`
	K8s       bool   `json:"k8s"`
	Promoted  bool   `json:"promoted"`
}

type Template struct {
	Key                 string   `json:"-"`
	DisplayName         string   `json:"displayName"`
	ExtendedDescription string   `json:"extendedDescription,omitempty"`
	AutomountFolders    []string `json:"automountFolders"`
	CleanupCommands     []string `json:"cleanupCommands"`
	OpenPorts           []int    `json:"openPorts"`
	StartupCommands     []string `json:"startupCommands"`
	StartupMinutes      int      `json:"startupMinutes,omitempty"`
	Version             int      `json:"version,omitempty"`
	Default             bool     `json:"default,omitempty"`
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
	UUID    string `json:"uuid"`
	Message string `json:"message"`
}

type DeleteInstanceResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

type Client struct {
	token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) ListInstances() ([]Instance, error) {
	req, err := http.NewRequest("GET", baseURL+"/instances/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed: invalid token")
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rawResponse map[string]Instance
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	instances := make([]Instance, 0, len(rawResponse))
	for id, instance := range rawResponse {
		instance.ID = id
		instances = append(instances, instance)
	}

	return instances, nil
}

func (c *Client) ListTemplates() ([]Template, error) {
	req, err := http.NewRequest("GET", baseURL+"/thunder-templates", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed: invalid token")
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rawResponse map[string]Template
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	templates := make([]Template, 0, len(rawResponse))
	for key, template := range rawResponse {
		template.Key = key
		templates = append(templates, template)
	}

	return templates, nil
}

func (c *Client) CreateInstance(req CreateInstanceRequest) (*CreateInstanceResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/instances/create", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed: invalid token")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var createResp CreateInstanceResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &createResp, nil
}

func (c *Client) DeleteInstance(instanceID string) (*DeleteInstanceResponse, error) {
	url := fmt.Sprintf("%s/instances/%s/delete", baseURL, instanceID)

	httpReq, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed: invalid token")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return &DeleteInstanceResponse{
		Message: string(body),
		Success: true,
	}, nil
}

// GetLatestBinaryHash fetches the latest Thunder virtualization binary hash
func (c *Client) GetLatestBinaryHash() (string, error) {
	metadataURL := "https://storage.googleapis.com/storage/v1/b/client-binary/o/client_linux_x86_64?alt=json"

	req, err := http.NewRequest("GET", metadataURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Metadata["hash"], nil
}

type AddSSHKeyResponse struct {
	UUID string `json:"uuid"`
	Key  string `json:"key"`
}

// AddSSHKey generates and adds SSH keypair to instance
func (c *Client) AddSSHKey(instanceID string) (*AddSSHKeyResponse, error) {
	url := fmt.Sprintf("%s/instances/%s/add_key", baseURL, instanceID)

	httpReq, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed: invalid token")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var keyResp AddSSHKeyResponse
	if err := json.Unmarshal(body, &keyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &keyResp, nil
}

type DeviceIDResponse struct {
	ID string `json:"id"`
}

// GetNextDeviceID requests a new device ID for GPU virtualization
func (c *Client) GetNextDeviceID() (string, error) {
	req, err := http.NewRequest("GET", baseURL+"/next_id", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("authentication failed: invalid token")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var deviceResp DeviceIDResponse
	if err := json.Unmarshal(body, &deviceResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return deviceResp.ID, nil
}

// ListInstancesWithIPUpdate fetches instances and updates their IP addresses
func (c *Client) ListInstancesWithIPUpdate() ([]Instance, error) {
	req, err := http.NewRequest("GET", baseURL+"/instances/list?update_ips=true", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed: invalid token")
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rawResponse map[string]Instance
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	instances := make([]Instance, 0, len(rawResponse))
	for id, instance := range rawResponse {
		instance.ID = id
		instances = append(instances, instance)
	}

	return instances, nil
}
