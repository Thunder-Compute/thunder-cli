package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/getsentry/sentry-go"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(token, baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	return c.httpClient.Do(req)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Thunder-Client", "GO-CLI")
}

func (c *Client) ValidateToken(ctx context.Context) error {
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "api",
		Message:  "validate_token",
		Level:    sentry.LevelInfo,
	})

	req, err := http.NewRequest("GET", c.baseURL+"/v1/auth/validate", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.do(ctx, req)
	if err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("api_method", "ValidateToken")
			scope.SetTag("api_url", c.baseURL)
			scope.SetLevel(sentry.LevelError)
			sentry.CaptureException(err)
		})
		return fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		err := fmt.Errorf("authentication failed: invalid token")
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("api_method", "ValidateToken")
			scope.SetTag("status_code", "401")
			scope.SetLevel(sentry.LevelWarning)
			sentry.CaptureException(err)
		})
		return err
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("token validation failed with status %d: %s", resp.StatusCode, string(body))
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("api_method", "ValidateToken")
			scope.SetTag("status_code", fmt.Sprintf("%d", resp.StatusCode))
			scope.SetExtra("response_body", string(body))
			scope.SetLevel(getLogLevelForStatus(resp.StatusCode))
			sentry.CaptureException(err)
		})
		return err
	}

	_, _ = io.ReadAll(resp.Body)
	return nil
}

func (c *Client) ListInstancesWithIPUpdateCtx(ctx context.Context) ([]Instance, error) {
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "api",
		Message:  "list_instances",
		Level:    sentry.LevelInfo,
	})

	req, err := http.NewRequest("GET", c.baseURL+"/instances/list?update_ips=true", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.do(ctx, req)
	if err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("api_method", "ListInstances")
			scope.SetTag("api_url", c.baseURL)
			scope.SetLevel(sentry.LevelError)
			sentry.CaptureException(err)
		})
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		err := fmt.Errorf("authentication failed: invalid token")
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("api_method", "ListInstances")
			scope.SetTag("status_code", "401")
			scope.SetLevel(sentry.LevelWarning)
			sentry.CaptureException(err)
		})
		return nil, err
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("api_method", "ListInstances")
			scope.SetTag("status_code", fmt.Sprintf("%d", resp.StatusCode))
			scope.SetLevel(getLogLevelForStatus(resp.StatusCode))
			sentry.CaptureException(err)
		})
		return nil, err
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

	// Sort instances by ID for consistent ordering
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].ID < instances[j].ID
	})

	return instances, nil
}

func (c *Client) AddSSHKeyCtx(ctx context.Context, instanceID string, req *AddSSHKeyRequest) (*AddSSHKeyResponse, error) {
	url := fmt.Sprintf("%s/instances/%s/add_key", c.baseURL, instanceID)

	var bodyReader io.Reader
	if req != nil {
		jsonData, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	httpReq, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.do(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle specific error cases with clear messages
	switch resp.StatusCode {
	case 200, 201:
		// Success - continue to parse
	case 401:
		return nil, fmt.Errorf("authentication failed: invalid token")
	case 400:
		// Parse error response for specific message
		var errResp struct {
			Code    string `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("%s", errResp.Message)
		}
		return nil, fmt.Errorf("invalid request: %s", string(body))
	case 404:
		return nil, fmt.Errorf("instance not found or not accessible")
	case 503:
		return nil, fmt.Errorf("instance is starting up and not ready yet - please wait and try again")
	default:
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var keyResp AddSSHKeyResponse
	if err := json.Unmarshal(body, &keyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &keyResp, nil
}

// TODO: Most likely just going to remove this
func (c *Client) GetNextDeviceIDCtx(ctx context.Context) (string, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/next_id", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.do(ctx, req)
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

	type deviceIDResponse struct {
		ID json.Number `json:"id"`
	}

	var deviceResp deviceIDResponse
	if err := json.Unmarshal(body, &deviceResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if deviceResp.ID == "" {
		return "", fmt.Errorf("response missing 'id' field")
	}

	idInt, err := deviceResp.ID.Int64()
	if err != nil {
		return "", fmt.Errorf("device ID must be an integer number, got: %q", string(deviceResp.ID))
	}

	deviceID := fmt.Sprintf("%d", idInt)
	return deviceID, nil
}

func (c *Client) ListInstancesWithIPUpdate() ([]Instance, error) {
	return c.ListInstancesWithIPUpdateCtx(context.Background())
}

func (c *Client) GetNextDeviceID() (string, error) {
	return c.GetNextDeviceIDCtx(context.Background())
}
func (c *Client) ListInstances() ([]Instance, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/instances/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

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

	// Sort instances by ID for consistent ordering
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].ID < instances[j].ID
	})

	return instances, nil
}

func (c *Client) ListTemplates() ([]Template, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/thunder-templates", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

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

	httpReq, err := http.NewRequest("POST", c.baseURL+"/instances/create", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

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
	url := fmt.Sprintf("%s/v1/instances/%s/delete", c.baseURL, instanceID)

	httpReq, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

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

// ModifyInstance modifies an existing instance configuration
func (c *Client) ModifyInstance(instanceID string, req InstanceModifyRequest) (*InstanceModifyResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/instances/%s/modify", c.baseURL, instanceID)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case 401:
		return nil, fmt.Errorf("authentication failed: invalid token")
	case 404:
		return nil, fmt.Errorf("instance not found")
	case 400:
		return nil, fmt.Errorf("invalid request: %s", string(body))
	case 409:
		return nil, fmt.Errorf("instance cannot be modified (may not be in RUNNING state)")
	case 200, 201, 202:
		// Success - continue to parse
	default:
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var modifyResp InstanceModifyResponse
	if err := json.Unmarshal(body, &modifyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &modifyResp, nil
}

// AddSSHKey adds an SSH key to an instance
// If req is nil, a new key pair will be generated
func (c *Client) AddSSHKey(instanceID string, req *AddSSHKeyRequest) (*AddSSHKeyResponse, error) {
	return c.AddSSHKeyCtx(context.Background(), instanceID, req)
}

// CreateSnapshot creates a snapshot from an instance
func (c *Client) CreateSnapshot(req CreateSnapshotRequest) (*CreateSnapshotResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/snapshots/create", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

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

	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 202 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var createResp CreateSnapshotResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &createResp, nil
}

// ListSnapshots retrieves all snapshots for the authenticated user
func (c *Client) ListSnapshots() (ListSnapshotsResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/v1/snapshots/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

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

	var snapshots ListSnapshotsResponse
	if err := json.Unmarshal(body, &snapshots); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return snapshots, nil
}

// DeleteSnapshot deletes a snapshot by ID
func (c *Client) DeleteSnapshot(snapshotID string) error {
	url := fmt.Sprintf("%s/v1/snapshots/%s", c.baseURL, snapshotID)

	httpReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed: invalid token")
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getLogLevelForStatus determines the appropriate Sentry level for HTTP status codes
func getLogLevelForStatus(statusCode int) sentry.Level {
	switch {
	case statusCode >= 500:
		return sentry.LevelError
	case statusCode >= 400:
		return sentry.LevelWarning
	default:
		return sentry.LevelInfo
	}
}
