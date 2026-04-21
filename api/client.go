package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/Thunder-Compute/thunder-cli/pkg/types"
	"github.com/getsentry/sentry-go"
	sentryhttpclient "github.com/getsentry/sentry-go/httpclient"
)

// ErrTransport marks transport-level HTTP failures where http.Client.Do could
// not complete the request — DNS failure, connection refused, connection
// reset, TLS handshake error, mid-stream EOF, etc. Callers use errors.Is to
// classify these uniformly as user/network noise, distinct from *APIError
// (which means the server did respond with a status >= 400).
var ErrTransport = errors.New("transport error")

// APIError is returned for HTTP responses with status >= 400 (except 401).
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

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
			Timeout:   30 * time.Second,
			Transport: sentryhttpclient.NewSentryRoundTripper(nil),
		},
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body, result interface{}) error {
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "api",
		Message:  method + " " + path,
		Level:    sentry.LevelInfo,
	})

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Thunder-Client", "GO-CLI")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: failed to make request: %w", ErrTransport, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("api_method", method)
			scope.SetTag("api_path", path)
			scope.SetTag("status_code", fmt.Sprintf("%d", resp.StatusCode))
			scope.SetLevel(sentry.LevelError)
			sentry.CaptureMessage(fmt.Sprintf("API server error: %s %s returned %d", method, path, resp.StatusCode))
		})
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		if resp.StatusCode == 401 {
			return &APIError{StatusCode: 401, Message: "authentication failed: invalid token"}
		}
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var parsed struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &parsed) == nil && parsed.Message != "" {
			apiErr.Message = parsed.Message
		} else {
			apiErr.Message = fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
		}
		return apiErr
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}
	return nil
}

// sortedInstances converts a map keyed by ID into a sorted slice.
func sortedInstances(raw map[string]Instance) []Instance {
	instances := make([]Instance, 0, len(raw))
	for id, inst := range raw {
		inst.ID = id
		instances = append(instances, inst)
	}
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].ID < instances[j].ID
	})
	return instances
}

func (c *Client) ValidateToken(ctx context.Context) (*ValidateTokenResult, error) {
	var result ValidateTokenResult
	if err := c.doRequest(ctx, "GET", "/v1/auth/validate", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListInstancesWithIPUpdateCtx(ctx context.Context) ([]Instance, error) {
	var raw map[string]Instance
	if err := c.doRequest(ctx, "GET", "/v1/instances/list?update_ips=true", nil, &raw); err != nil {
		return nil, err
	}
	return sortedInstances(raw), nil
}

func (c *Client) ListInstancesWithIPUpdate() ([]Instance, error) {
	return c.ListInstancesWithIPUpdateCtx(context.Background())
}

func (c *Client) ListInstances() ([]Instance, error) {
	var raw map[string]Instance
	if err := c.doRequest(context.Background(), "GET", "/v1/instances/list", nil, &raw); err != nil {
		return nil, err
	}
	return sortedInstances(raw), nil
}

func (c *Client) AddSSHKeyCtx(ctx context.Context, instanceID string) (*AddSSHKeyResponse, error) {
	var resp AddSSHKeyResponse
	if err := c.doRequest(ctx, "POST", fmt.Sprintf("/v1/instances/%s/add_key", instanceID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) AddSSHKey(instanceID string) (*AddSSHKeyResponse, error) {
	return c.AddSSHKeyCtx(context.Background(), instanceID)
}

func (c *Client) ListTemplates() ([]TemplateEntry, error) {
	var raw types.ThunderTemplatesResponse
	if err := c.doRequest(context.Background(), "GET", "/v1/thunder-templates", nil, &raw); err != nil {
		return nil, err
	}
	entries := make([]TemplateEntry, 0, len(raw))
	for key, tmpl := range raw {
		entries = append(entries, TemplateEntry{Key: key, Template: tmpl})
	}
	return entries, nil
}

func (c *Client) CreateInstance(req CreateInstanceRequest) (*CreateInstanceResponse, error) {
	var resp CreateInstanceResponse
	if err := c.doRequest(context.Background(), "POST", "/v1/instances/create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteInstance(instanceID string) (*DeleteInstanceResponse, error) {
	if err := c.doRequest(context.Background(), "POST", fmt.Sprintf("/v1/instances/%s/delete", instanceID), nil, nil); err != nil {
		return nil, err
	}
	return &DeleteInstanceResponse{Message: "Instance deleted successfully", Success: true}, nil
}

// ModifyInstance modifies an existing instance configuration.
func (c *Client) ModifyInstance(instanceID string, req InstanceModifyRequest) (*InstanceModifyResponse, error) {
	var resp InstanceModifyResponse
	err := c.doRequest(context.Background(), "POST", fmt.Sprintf("/v1/instances/%s/modify", instanceID), req, &resp)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			switch apiErr.StatusCode {
			case 404:
				return nil, fmt.Errorf("instance not found")
			case 409:
				return nil, fmt.Errorf("instance cannot be modified (may not be in RUNNING state)")
			}
		}
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateSnapshot(req CreateSnapshotRequest) (*CreateSnapshotResponse, error) {
	var resp CreateSnapshotResponse
	if err := c.doRequest(context.Background(), "POST", "/v1/snapshots/create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListSnapshots() (ListSnapshotsResponse, error) {
	var resp ListSnapshotsResponse
	if err := c.doRequest(context.Background(), "GET", "/v1/snapshots/list", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) DeleteSnapshot(snapshotID string) error {
	return c.doRequest(context.Background(), "DELETE", fmt.Sprintf("/v1/snapshots/%s", snapshotID), nil, nil)
}

// FetchPricing retrieves the public pricing data from the API.
func (c *Client) FetchPricing() (map[string]float64, error) {
	var result struct {
		Pricing map[string]float64 `json:"pricing"`
	}
	if err := c.doRequest(context.Background(), "GET", "/v1/pricing", nil, &result); err != nil {
		return nil, err
	}
	return result.Pricing, nil
}

// GetSpecs retrieves GPU spec configurations from the API.
func (c *Client) GetSpecs() (map[string]GpuSpecConfig, error) {
	var result struct {
		Specs map[string]GpuSpecConfig `json:"specs"`
	}
	if err := c.doRequest(context.Background(), "GET", "/v1/specs", nil, &result); err != nil {
		return nil, err
	}
	return result.Specs, nil
}
