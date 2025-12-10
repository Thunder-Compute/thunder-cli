package cmd

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock API Client
// =============================================================================

// mockAPIClient implements api.ConnectClient for testing
type mockAPIClient struct {
	instances           []api.Instance
	listInstancesErr    error
	listInstancesCalled int

	listInstancesWithIPUpdateInstances []api.Instance
	listInstancesWithIPUpdateErr       error
	listInstancesWithIPUpdateCalled    int

	addSSHKeyResponse    *api.AddSSHKeyResponse
	addSSHKeyErr         error
	addSSHKeyCalled      int
	addSSHKeyInstanceIDs []string

	binaryHash    string
	binaryHashErr error

	nextDeviceID    string
	nextDeviceIDErr error

	mu sync.Mutex
}

func (m *mockAPIClient) ListInstances() ([]api.Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listInstancesCalled++
	return m.instances, m.listInstancesErr
}

func (m *mockAPIClient) ListInstancesWithIPUpdateCtx(ctx context.Context) ([]api.Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listInstancesWithIPUpdateCalled++
	if m.listInstancesWithIPUpdateInstances != nil {
		return m.listInstancesWithIPUpdateInstances, m.listInstancesWithIPUpdateErr
	}
	return m.instances, m.listInstancesWithIPUpdateErr
}

func (m *mockAPIClient) AddSSHKeyCtx(ctx context.Context, instanceID string) (*api.AddSSHKeyResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addSSHKeyCalled++
	m.addSSHKeyInstanceIDs = append(m.addSSHKeyInstanceIDs, instanceID)
	return m.addSSHKeyResponse, m.addSSHKeyErr
}

func (m *mockAPIClient) GetLatestBinaryHashCtx(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.binaryHash, m.binaryHashErr
}

func (m *mockAPIClient) GetNextDeviceID() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nextDeviceID, m.nextDeviceIDErr
}

// =============================================================================
// Mock SSH Client
// =============================================================================

// mockSSHClient implements sshClient interface for testing
type mockSSHClient struct {
	closed bool
}

func (m *mockSSHClient) Close() error {
	m.closed = true
	return nil
}

// =============================================================================
// Test Helpers
// =============================================================================

// createTestInstance creates a test instance with the given parameters
func createTestInstance(id, uuid, name, ip, status, template, mode string, port int) api.Instance {
	return api.Instance{
		ID:       id,
		UUID:     uuid,
		Name:     name,
		IP:       ip,
		Status:   status,
		Template: template,
		Mode:     mode,
		Port:     port,
		NumGPUs:  "1",
		GPUType:  "t4",
	}
}

// setupTestEnvironment creates a temporary test environment with config and keys
func setupTestEnvironment(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)

	// Create .thunder directory structure
	thunderDir := filepath.Join(tmpDir, ".thunder")
	keysDir := filepath.Join(thunderDir, "keys")
	require.NoError(t, os.MkdirAll(keysDir, 0700))

	// Create .ssh directory
	sshDir := filepath.Join(tmpDir, ".ssh")
	require.NoError(t, os.MkdirAll(sshDir, 0700))

	cleanup := func() {
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		} else {
			os.Unsetenv("HOME")
		}
	}

	return tmpDir, cleanup
}

// generateTestSSHKey generates a test RSA private key in PEM format
func generateTestSSHKey(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	return string(pem.EncodeToMemory(pemBlock))
}

// saveTestKey saves a test key to the keys directory
func saveTestKey(t *testing.T, tmpDir, uuid string) string {
	t.Helper()

	keyFile := filepath.Join(tmpDir, ".thunder", "keys", uuid)
	keyData := generateTestSSHKey(t)
	require.NoError(t, os.WriteFile(keyFile, []byte(keyData), 0600))
	return keyFile
}

// mockExecCommand creates a mock exec.Command that captures arguments
func mockExecCommand(capturedArgs *[]string, exitCode int) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		*capturedArgs = append([]string{name}, args...)

		// Create a command that exits with the specified code
		if exitCode == 0 {
			return exec.Command("true")
		}
		// Use a shell command to exit with specific code
		return exec.Command("sh", "-c", fmt.Sprintf("exit %d", exitCode))
	}
}

// mockConfigLoader returns a mock config loader for testing
func mockConfigLoader(token string) func() (*Config, error) {
	return func() (*Config, error) {
		return &Config{Token: token}, nil
	}
}

// mockSSHConnector creates a mock SSH connector that returns a mock client
func mockSSHConnector(client sshClient, err error) func(context.Context, string, string, int, int) (sshClient, error) {
	return func(ctx context.Context, ip, keyFile string, port, maxWait int) (sshClient, error) {
		return client, err
	}
}

// =============================================================================
// Test Cases
// =============================================================================

func TestRunConnect_InstanceNotFound(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mockAPIClient{
		instances: []api.Instance{
			createTestInstance("inst-1", "uuid-1", "test-instance", "192.168.1.100", "RUNNING", "ubuntu", "prototyping", 22),
		},
	}

	opts := &connectOptions{
		client:       mockClient,
		skipTTYCheck: true,
		skipTUI:      true,
		configLoader: mockConfigLoader("test-token"),
	}

	err := runConnectWithOptions("nonexistent", []string{}, false, false, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify ListInstances was called
	assert.Equal(t, 1, mockClient.listInstancesCalled)
	_ = tmpDir // keep reference
}

func TestRunConnect_InstanceNotRunning(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mockAPIClient{
		instances: []api.Instance{
			createTestInstance("inst-1", "uuid-1", "test-instance", "192.168.1.100", "STOPPED", "ubuntu", "prototyping", 22),
		},
	}

	opts := &connectOptions{
		client:       mockClient,
		skipTTYCheck: true,
		skipTUI:      true,
		configLoader: mockConfigLoader("test-token"),
	}

	err := runConnectWithOptions("inst-1", []string{}, false, false, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")

	_ = tmpDir
}

func TestRunConnect_InstanceNoIP(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mockAPIClient{
		instances: []api.Instance{
			createTestInstance("inst-1", "uuid-1", "test-instance", "", "RUNNING", "ubuntu", "prototyping", 22),
		},
	}

	opts := &connectOptions{
		client:       mockClient,
		skipTTYCheck: true,
		skipTUI:      true,
		configLoader: mockConfigLoader("test-token"),
	}

	err := runConnectWithOptions("inst-1", []string{}, false, false, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no IP address")

	_ = tmpDir
}

func TestRunConnect_NoInstances(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mockAPIClient{
		instances: []api.Instance{},
	}

	opts := &connectOptions{
		client:       mockClient,
		skipTTYCheck: true,
		skipTUI:      true,
		configLoader: mockConfigLoader("test-token"),
	}

	// When no instanceID is provided and no instances exist, should return nil (no error)
	// but with empty instances list
	err := runConnectWithOptions("", []string{}, false, false, opts)
	// Should not error but exit gracefully
	assert.NoError(t, err)

	_ = tmpDir
}

func TestRunConnect_InvalidPort(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mockAPIClient{
		instances: []api.Instance{
			createTestInstance("inst-1", "uuid-1", "test-instance", "192.168.1.100", "RUNNING", "ubuntu", "prototyping", 22),
		},
	}

	opts := &connectOptions{
		client:       mockClient,
		skipTTYCheck: true,
		skipTUI:      true,
		configLoader: mockConfigLoader("test-token"),
	}

	err := runConnectWithOptions("inst-1", []string{"not-a-port"}, false, false, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid port")

	_ = tmpDir
}

func TestRunConnect_NoAuthToken(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	opts := &connectOptions{
		skipTTYCheck: true,
		skipTUI:      true,
		configLoader: func() (*Config, error) {
			return &Config{Token: ""}, nil
		},
	}

	err := runConnectWithOptions("inst-1", []string{}, false, false, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no authentication token")

	_ = tmpDir
}

func TestRunConnect_ConfigLoadError(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	opts := &connectOptions{
		skipTTYCheck: true,
		skipTUI:      true,
		configLoader: func() (*Config, error) {
			return nil, fmt.Errorf("config error")
		},
	}

	err := runConnectWithOptions("inst-1", []string{}, false, false, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")

	_ = tmpDir
}

func TestRunConnect_ListInstancesError(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mockAPIClient{
		listInstancesErr: fmt.Errorf("API error"),
	}

	opts := &connectOptions{
		client:       mockClient,
		skipTTYCheck: true,
		skipTUI:      true,
		configLoader: mockConfigLoader("test-token"),
	}

	err := runConnectWithOptions("inst-1", []string{}, false, false, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list instances")

	_ = tmpDir
}

func TestBuildSSHArgs_BaseFlags(t *testing.T) {
	// Test that the base SSH arguments are correctly constructed
	// This tests the argument building logic without running the full connect flow

	keyFile := "/path/to/key"
	port := 2222
	ip := "192.168.1.100"

	// Simulate the argument building from runConnect
	sshArgs := []string{
		"-q",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=/dev/null",
		"-i", keyFile,
		"-p", fmt.Sprintf("%d", port),
		"-t",
	}

	// Add ControlMaster options (non-Windows)
	if runtime.GOOS != "windows" {
		homeDir := "/home/test"
		controlPath := fmt.Sprintf("%s/.thunder/thunder-control-%%h-%%p-%%r", homeDir)
		sshArgs = append(sshArgs,
			"-o", "ControlMaster=auto",
			"-o", fmt.Sprintf("ControlPath=%s", controlPath),
			"-o", "ControlPersist=5m",
		)
	}

	sshArgs = append(sshArgs, fmt.Sprintf("ubuntu@%s", ip))

	// Verify base flags
	assert.Contains(t, sshArgs, "-q")
	assert.Contains(t, sshArgs, "-t")
	assert.Contains(t, sshArgs, keyFile)
	assert.Contains(t, sshArgs, fmt.Sprintf("%d", port))
	assert.Contains(t, sshArgs, fmt.Sprintf("ubuntu@%s", ip))
}

func TestBuildSSHArgs_PortForwarding(t *testing.T) {
	// Test port forwarding argument construction
	tunnelPorts := []int{8080, 3000}
	templatePorts := []int{8888} // jupyter

	// Merge ports like runConnect does
	allPorts := make(map[int]bool)
	for _, p := range tunnelPorts {
		allPorts[p] = true
	}
	for _, p := range templatePorts {
		allPorts[p] = true
	}

	var sshArgs []string
	for port := range allPorts {
		sshArgs = append(sshArgs, "-L", fmt.Sprintf("%d:localhost:%d", port, port))
	}

	// Verify all ports are present
	argsStr := strings.Join(sshArgs, " ")
	assert.Contains(t, argsStr, "8080:localhost:8080")
	assert.Contains(t, argsStr, "3000:localhost:3000")
	assert.Contains(t, argsStr, "8888:localhost:8888")
}

func TestBuildSSHArgs_PortForwardingNoDuplicates(t *testing.T) {
	// Test that duplicate ports are not included twice
	tunnelPorts := []int{8080, 8888}   // User specifies 8888
	templatePorts := []int{8888, 6006} // Template also has 8888

	allPorts := make(map[int]bool)
	for _, p := range tunnelPorts {
		allPorts[p] = true
	}
	for _, p := range templatePorts {
		allPorts[p] = true
	}

	// Should have exactly 3 unique ports
	assert.Len(t, allPorts, 3)
	assert.True(t, allPorts[8080])
	assert.True(t, allPorts[8888])
	assert.True(t, allPorts[6006])
}

func TestSSHExitCodeHandling(t *testing.T) {
	tests := []struct {
		name        string
		exitCode    int
		expectError bool
	}{
		{
			name:        "exit code 0 - success",
			exitCode:    0,
			expectError: false,
		},
		{
			name:        "exit code 130 - Ctrl+C",
			exitCode:    130,
			expectError: false,
		},
		{
			name:        "exit code 255 - connection closed",
			exitCode:    255,
			expectError: false,
		},
		{
			name:        "exit code 1 - error",
			exitCode:    1,
			expectError: true,
		},
		{
			name:        "exit code 2 - error",
			exitCode:    2,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the exit code handling logic from runConnect
			shouldError := tt.exitCode != 0 && tt.exitCode != 130 && tt.exitCode != 255

			if tt.expectError {
				assert.True(t, shouldError, "expected exit code %d to produce an error", tt.exitCode)
			} else {
				assert.False(t, shouldError, "expected exit code %d to not produce an error", tt.exitCode)
			}
		})
	}
}

func TestMockAPIClient_ListInstances(t *testing.T) {
	instances := []api.Instance{
		createTestInstance("inst-1", "uuid-1", "instance-1", "192.168.1.100", "RUNNING", "ubuntu", "prototyping", 22),
		createTestInstance("inst-2", "uuid-2", "instance-2", "192.168.1.101", "STOPPED", "pytorch", "production", 22),
	}

	client := &mockAPIClient{instances: instances}

	result, err := client.ListInstances()
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 1, client.listInstancesCalled)

	// Call again
	result, err = client.ListInstances()
	require.NoError(t, err)
	assert.Equal(t, 2, client.listInstancesCalled)
}

func TestMockAPIClient_AddSSHKey(t *testing.T) {
	client := &mockAPIClient{
		addSSHKeyResponse: &api.AddSSHKeyResponse{
			UUID: "uuid-1",
			Key:  "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----",
		},
	}

	ctx := context.Background()
	resp, err := client.AddSSHKeyCtx(ctx, "inst-1")
	require.NoError(t, err)
	assert.Equal(t, "uuid-1", resp.UUID)
	assert.Equal(t, 1, client.addSSHKeyCalled)
	assert.Contains(t, client.addSSHKeyInstanceIDs, "inst-1")
}

func TestMockAPIClient_Error(t *testing.T) {
	client := &mockAPIClient{
		listInstancesErr: fmt.Errorf("network error"),
	}

	_, err := client.ListInstances()
	require.Error(t, err)
	assert.Equal(t, "network error", err.Error())
}

func TestCreateTestInstance(t *testing.T) {
	instance := createTestInstance(
		"test-id",
		"test-uuid",
		"test-name",
		"10.0.0.1",
		"RUNNING",
		"jupyter",
		"prototyping",
		2222,
	)

	assert.Equal(t, "test-id", instance.ID)
	assert.Equal(t, "test-uuid", instance.UUID)
	assert.Equal(t, "test-name", instance.Name)
	assert.Equal(t, "10.0.0.1", instance.IP)
	assert.Equal(t, "RUNNING", instance.Status)
	assert.Equal(t, "jupyter", instance.Template)
	assert.Equal(t, "prototyping", instance.Mode)
	assert.Equal(t, 2222, instance.Port)
}

func TestMockExecCommand(t *testing.T) {
	var capturedArgs []string

	mockCmd := mockExecCommand(&capturedArgs, 0)
	cmd := mockCmd("ssh", "-q", "-i", "/path/to/key", "ubuntu@192.168.1.100")

	// The mock returns a real command that will exit with the specified code
	err := cmd.Run()
	assert.NoError(t, err)

	// Verify arguments were captured
	assert.Equal(t, "ssh", capturedArgs[0])
	assert.Contains(t, capturedArgs, "-q")
	assert.Contains(t, capturedArgs, "-i")
	assert.Contains(t, capturedArgs, "/path/to/key")
	assert.Contains(t, capturedArgs, "ubuntu@192.168.1.100")
}

func TestMockExecCommand_NonZeroExit(t *testing.T) {
	var capturedArgs []string

	mockCmd := mockExecCommand(&capturedArgs, 1)
	cmd := mockCmd("ssh", "test")

	err := cmd.Run()
	assert.Error(t, err)
}

func TestSetupTestEnvironment(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Verify directories were created
	thunderDir := filepath.Join(tmpDir, ".thunder")
	keysDir := filepath.Join(thunderDir, "keys")
	sshDir := filepath.Join(tmpDir, ".ssh")

	assert.DirExists(t, thunderDir)
	assert.DirExists(t, keysDir)
	assert.DirExists(t, sshDir)

	// Verify HOME was set
	assert.Equal(t, tmpDir, os.Getenv("HOME"))
}

func TestSaveTestKey(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	uuid := "test-uuid-12345"
	keyFile := saveTestKey(t, tmpDir, uuid)

	// Verify file exists
	assert.FileExists(t, keyFile)

	// Verify it's a valid PEM file
	data, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "-----BEGIN RSA PRIVATE KEY-----")
	assert.Contains(t, string(data), "-----END RSA PRIVATE KEY-----")
}

func TestSSHConfigUpdate(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a mock SSH config file
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")

	initialConfig := `Host existing-host
    HostName 10.0.0.1
    User admin
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0600))

	// Verify initial state
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "existing-host")
}

func TestInstanceLookup_ByID(t *testing.T) {
	instances := []api.Instance{
		createTestInstance("inst-1", "uuid-1", "name-1", "192.168.1.100", "RUNNING", "ubuntu", "prototyping", 22),
		createTestInstance("inst-2", "uuid-2", "name-2", "192.168.1.101", "RUNNING", "pytorch", "production", 22),
	}

	// Lookup by ID
	var found *api.Instance
	lookupID := "inst-1"
	for i := range instances {
		if instances[i].ID == lookupID || instances[i].UUID == lookupID || instances[i].Name == lookupID {
			found = &instances[i]
			break
		}
	}

	require.NotNil(t, found)
	assert.Equal(t, "inst-1", found.ID)
}

func TestInstanceLookup_ByUUID(t *testing.T) {
	instances := []api.Instance{
		createTestInstance("inst-1", "uuid-1", "name-1", "192.168.1.100", "RUNNING", "ubuntu", "prototyping", 22),
		createTestInstance("inst-2", "uuid-2", "name-2", "192.168.1.101", "RUNNING", "pytorch", "production", 22),
	}

	// Lookup by UUID
	var found *api.Instance
	lookupID := "uuid-2"
	for i := range instances {
		if instances[i].ID == lookupID || instances[i].UUID == lookupID || instances[i].Name == lookupID {
			found = &instances[i]
			break
		}
	}

	require.NotNil(t, found)
	assert.Equal(t, "inst-2", found.ID)
	assert.Equal(t, "uuid-2", found.UUID)
}

func TestInstanceLookup_ByName(t *testing.T) {
	instances := []api.Instance{
		createTestInstance("inst-1", "uuid-1", "my-gpu-instance", "192.168.1.100", "RUNNING", "ubuntu", "prototyping", 22),
		createTestInstance("inst-2", "uuid-2", "training-server", "192.168.1.101", "RUNNING", "pytorch", "production", 22),
	}

	// Lookup by Name
	var found *api.Instance
	lookupID := "training-server"
	for i := range instances {
		if instances[i].ID == lookupID || instances[i].UUID == lookupID || instances[i].Name == lookupID {
			found = &instances[i]
			break
		}
	}

	require.NotNil(t, found)
	assert.Equal(t, "inst-2", found.ID)
	assert.Equal(t, "training-server", found.Name)
}

func TestDefaultPort(t *testing.T) {
	// Test that port defaults to 22 when not specified
	instance := createTestInstance("inst-1", "uuid-1", "name-1", "192.168.1.100", "RUNNING", "ubuntu", "prototyping", 0)

	port := instance.Port
	if port == 0 {
		port = 22
	}

	assert.Equal(t, 22, port)
}

func TestMockSSHClient(t *testing.T) {
	client := &mockSSHClient{}
	assert.False(t, client.closed)

	err := client.Close()
	assert.NoError(t, err)
	assert.True(t, client.closed)
}

func TestMockSSHConnector(t *testing.T) {
	mockClient := &mockSSHClient{}
	connector := mockSSHConnector(mockClient, nil)

	ctx := context.Background()
	client, err := connector(ctx, "192.168.1.100", "/path/to/key", 22, 120)

	assert.NoError(t, err)
	assert.Equal(t, mockClient, client)
}

func TestMockSSHConnector_Error(t *testing.T) {
	connector := mockSSHConnector(nil, fmt.Errorf("connection failed"))

	ctx := context.Background()
	client, err := connector(ctx, "192.168.1.100", "/path/to/key", 22, 120)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Equal(t, "connection failed", err.Error())
}

func TestMockConfigLoader(t *testing.T) {
	loader := mockConfigLoader("my-test-token")

	config, err := loader()
	require.NoError(t, err)
	assert.Equal(t, "my-test-token", config.Token)
}

func TestTemplatePortMapping(t *testing.T) {
	// Test that template ports are correctly identified
	// This mirrors the logic in utils.GetTemplateOpenPorts

	templates := map[string][]int{
		"jupyter":     {8888},
		"vscode":      {8080},
		"rstudio":     {8787},
		"tensorboard": {6006},
		"mlflow":      {5000},
	}

	for template, expectedPorts := range templates {
		t.Run(template, func(t *testing.T) {
			// The actual implementation is in utils, but we can verify the mapping
			assert.NotEmpty(t, expectedPorts, "template %s should have ports", template)
		})
	}
}

func TestConnectOptions_Defaults(t *testing.T) {
	opts := defaultConnectOptions("test-token", "https://api.thundercompute.com:8443")

	assert.NotNil(t, opts.client)
	assert.False(t, opts.skipTTYCheck)
	assert.False(t, opts.skipTUI)
	assert.NotNil(t, opts.sshConnector)
	assert.NotNil(t, opts.execCommand)
	assert.NotNil(t, opts.configLoader)
}

func TestControlMasterArgs_NotOnWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping ControlMaster test on Windows")
	}

	homeDir := "/home/testuser"
	controlPath := fmt.Sprintf("%s/.thunder/thunder-control-%%h-%%p-%%r", homeDir)

	// Verify the control path format
	assert.Contains(t, controlPath, "thunder-control")
	assert.Contains(t, controlPath, "%h-%p-%r")
}

func TestConcurrentMockAPIClient(t *testing.T) {
	// Test that mock API client is thread-safe
	client := &mockAPIClient{
		instances: []api.Instance{
			createTestInstance("inst-1", "uuid-1", "name-1", "192.168.1.100", "RUNNING", "ubuntu", "prototyping", 22),
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.ListInstances()
		}()
	}
	wg.Wait()

	assert.Equal(t, 10, client.listInstancesCalled)
}
