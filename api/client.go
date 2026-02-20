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

	"github.com/Thunder-Compute/thunder-cli/pkg/types"
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

	req, err := http.NewRequest("GET", c.baseURL+"/v1/instances/list?update_ips=true", nil)
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

func (c *Client) AddSSHKeyCtx(ctx context.Context, instanceID string) (*AddSSHKeyResponse, error) {
	url := fmt.Sprintf("%s/v1/instances/%s/add_key", c.baseURL, instanceID)

	httpReq, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.do(ctx, httpReq)
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

func (c *Client) ListInstancesWithIPUpdate() ([]Instance, error) {
	return c.ListInstancesWithIPUpdateCtx(context.Background())
}

func (c *Client) ListInstances() ([]Instance, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/v1/instances/list", nil)
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

func (c *Client) ListTemplates() ([]TemplateEntry, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/v1/thunder-templates", nil)
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

	var rawResponse types.ThunderTemplatesResponse
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	entries := make([]TemplateEntry, 0, len(rawResponse))
	for key, tmpl := range rawResponse {
		entries = append(entries, TemplateEntry{Key: key, Template: tmpl})
	}

	return entries, nil
}

func (c *Client) CreateInstance(req CreateInstanceRequest) (*CreateInstanceResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/instances/create", bytes.NewBuffer(jsonData))
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

// AddSSHKey generates and adds SSH keypair to instance
func (c *Client) AddSSHKey(instanceID string) (*AddSSHKeyResponse, error) {
	return c.AddSSHKeyCtx(context.Background(), instanceID)
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

// ListSSHKeys retrieves all SSH keys for the authenticated user's organization
func (c *Client) ListSSHKeys() (SSHKeyListResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/v1/keys/list", nil)
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

	var keys SSHKeyListResponse
	if err := json.Unmarshal(body, &keys); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return keys, nil
}

// AddSSHKeyToOrg adds an SSH public key to the user's organization
func (c *Client) AddSSHKeyToOrg(name, publicKey string) (*SSHKeyAddResponse, error) {
	reqBody := SSHKeyAddRequest{
		Name:      name,
		PublicKey: publicKey,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/keys/add", bytes.NewBuffer(jsonData))
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

	var addResp SSHKeyAddResponse
	if err := json.Unmarshal(body, &addResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &addResp, nil
}

// DeleteSSHKey deletes an SSH key by ID
func (c *Client) DeleteSSHKey(keyID string) error {
	url := fmt.Sprintf("%s/v1/keys/%s", c.baseURL, keyID)

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

// AddSSHKeyToInstanceWithPublicKey adds an existing public key to an instance
func (c *Client) AddSSHKeyToInstanceWithPublicKey(instanceID, publicKey string) (*AddSSHKeyResponse, error) {
	reqBody := struct {
		PublicKey string `json:"public_key"`
	}{PublicKey: publicKey}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/instances/%s/add_key", c.baseURL, instanceID)
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
