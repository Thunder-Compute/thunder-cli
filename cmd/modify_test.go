package cmd

import (
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// baseInstance returns a typical prototyping A6000 instance for testing.
func baseInstance() *api.Instance {
	return &api.Instance{
		ID:       "inst-123",
		Mode:     "prototyping",
		GPUType:  "a6000",
		NumGPUs:  "1",
		CPUCores: "4",
		Storage:  100,
		Status:   "RUNNING",
	}
}

// productionInstance returns a typical production A100 instance for testing.
func productionInstance() *api.Instance {
	return &api.Instance{
		ID:       "inst-456",
		Mode:     "production",
		GPUType:  "a100xl",
		NumGPUs:  "1",
		CPUCores: "18",
		Storage:  100,
		Status:   "RUNNING",
	}
}

func TestValidateAndBuildModifyRequest(t *testing.T) {
	specs := testSpecStore()

	tests := []struct {
		name          string
		presets       *tui.ModifyPresets
		instance      *api.Instance
		expectError   bool
		errorContains string
		checkReq      func(t *testing.T, req api.InstanceModifyRequest)
	}{
		{
			name:          "empty presets returns error",
			presets:       &tui.ModifyPresets{},
			instance:      baseInstance(),
			expectError:   true,
			errorContains: "no changes specified",
		},
		{
			name:    "valid vCPU change in prototyping",
			presets: &tui.ModifyPresets{VCPUs: intPtr(8)},
			instance: baseInstance(),
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.CPUCores)
				assert.Equal(t, 8, *req.CPUCores)
				assert.Nil(t, req.Mode)
				assert.Nil(t, req.GPUType)
			},
		},
		{
			name:          "invalid vCPU for GPU type",
			presets:       &tui.ModifyPresets{VCPUs: intPtr(6)},
			instance:      baseInstance(),
			expectError:   true,
			errorContains: "vcpus must be one of [4 8] for a6000 with 1 GPU(s)",
		},
		{
			name:          "vcpus in production mode rejected",
			presets:       &tui.ModifyPresets{VCPUs: intPtr(8)},
			instance:      productionInstance(),
			expectError:   true,
			errorContains: "production mode does not use --vcpus flag",
		},
		{
			name:          "invalid mode string",
			presets:       &tui.ModifyPresets{Mode: strPtr("invalid")},
			instance:      baseInstance(),
			expectError:   true,
			errorContains: "mode must be 'prototyping' or 'production'",
		},
		{
			name:          "switch to production without num-gpus",
			presets:       &tui.ModifyPresets{Mode: strPtr("production")},
			instance:      baseInstance(),
			expectError:   true,
			errorContains: "switching to production requires --num-gpus",
		},
		{
			name:          "switch to prototyping without vcpus",
			presets:       &tui.ModifyPresets{Mode: strPtr("prototyping")},
			instance:      productionInstance(),
			expectError:   true,
			errorContains: "switching to prototyping requires --vcpus",
		},
		{
			name: "valid switch to production with num-gpus",
			presets: &tui.ModifyPresets{
				Mode:    strPtr("production"),
				GPUType: strPtr("a100"),
				NumGPUs: intPtr(1),
			},
			instance: baseInstance(),
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.Mode)
				assert.Equal(t, api.InstanceMode("production"), *req.Mode)
				require.NotNil(t, req.NumGPUs)
				assert.Equal(t, 1, *req.NumGPUs)
			},
		},
		{
			name: "valid switch to prototyping with vcpus",
			presets: &tui.ModifyPresets{
				Mode:  strPtr("prototyping"),
				VCPUs: intPtr(4),
			},
			instance: productionInstance(),
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.Mode)
				assert.Equal(t, api.InstanceMode("prototyping"), *req.Mode)
				require.NotNil(t, req.CPUCores)
				assert.Equal(t, 4, *req.CPUCores)
			},
		},
		{
			name:    "GPU type alias normalized (a100 -> a100xl)",
			presets: &tui.ModifyPresets{GPUType: strPtr("a100")},
			instance: baseInstance(),
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.GPUType)
				assert.Equal(t, "a100xl", *req.GPUType)
			},
		},
		{
			name:          "invalid GPU type for mode",
			presets:       &tui.ModifyPresets{GPUType: strPtr("a6000")},
			instance:      productionInstance(),
			expectError:   true,
			errorContains: "invalid GPU type 'a6000' for production mode",
		},
		{
			name:          "invalid num-gpus for GPU type",
			presets:       &tui.ModifyPresets{NumGPUs: intPtr(3)},
			instance:      baseInstance(),
			expectError:   true,
			errorContains: "num-gpus 3 is not valid for a6000 prototyping",
		},
		{
			name:    "valid disk size increase",
			presets: &tui.ModifyPresets{DiskSizeGB: intPtr(200)},
			instance: baseInstance(),
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.DiskSizeGB)
				assert.Equal(t, 200, *req.DiskSizeGB)
			},
		},
		{
			name:          "disk size cannot shrink",
			presets:       &tui.ModifyPresets{DiskSizeGB: intPtr(50)},
			instance:      baseInstance(),
			expectError:   true,
			errorContains: "disk size cannot be smaller than current size (100 GB)",
		},
		{
			name:          "disk size exceeds max for GPU type",
			presets:       &tui.ModifyPresets{DiskSizeGB: intPtr(500)},
			instance:      baseInstance(), // a6000 max is 300
			expectError:   true,
			errorContains: "disk size must be between 100 and 300 GB",
		},
		{
			name: "same mode is a valid change (not a switch)",
			presets: &tui.ModifyPresets{
				Mode: strPtr("prototyping"),
			},
			instance: baseInstance(),
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.Mode)
				assert.Equal(t, api.InstanceMode("prototyping"), *req.Mode)
			},
		},
		{
			name: "multiple valid changes",
			presets: &tui.ModifyPresets{
				GPUType:    strPtr("h100"),
				VCPUs:      intPtr(8),
				DiskSizeGB: intPtr(200),
			},
			instance: baseInstance(),
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.GPUType)
				assert.Equal(t, "h100", *req.GPUType)
				require.NotNil(t, req.CPUCores)
				assert.Equal(t, 8, *req.CPUCores)
				require.NotNil(t, req.DiskSizeGB)
				assert.Equal(t, 200, *req.DiskSizeGB)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := validateAndBuildModifyRequest(tt.presets, tt.instance, specs)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.checkReq != nil {
					tt.checkReq(t, req)
				}
			}
		})
	}
}

func TestBuildModifyRequestFromConfig(t *testing.T) {
	instance := baseInstance()

	tests := []struct {
		name          string
		config        *tui.ModifyConfig
		expectError   bool
		errorContains string
		checkReq      func(t *testing.T, req api.InstanceModifyRequest)
	}{
		{
			name: "no changes returns error",
			config: &tui.ModifyConfig{
				ModeChanged:    false,
				GPUChanged:     false,
				ComputeChanged: false,
				DiskChanged:    false,
			},
			expectError:   true,
			errorContains: "no changes specified",
		},
		{
			name: "mode change sets mode",
			config: &tui.ModifyConfig{
				Mode:        "production",
				ModeChanged: true,
			},
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.Mode)
				assert.Equal(t, api.InstanceMode("production"), *req.Mode)
			},
		},
		{
			name: "prototyping compute change sets CPUCores",
			config: &tui.ModifyConfig{
				VCPUs:          8,
				ComputeChanged: true,
			},
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.CPUCores)
				assert.Equal(t, 8, *req.CPUCores)
				assert.Nil(t, req.NumGPUs)
			},
		},
		{
			name: "production compute change sets NumGPUs",
			config: &tui.ModifyConfig{
				Mode:           "production",
				ModeChanged:    true,
				NumGPUs:        2,
				ComputeChanged: true,
			},
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.NumGPUs)
				assert.Equal(t, 2, *req.NumGPUs)
				assert.Nil(t, req.CPUCores)
			},
		},
		{
			name: "disk change",
			config: &tui.ModifyConfig{
				DiskSizeGB:  300,
				DiskChanged: true,
			},
			checkReq: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.DiskSizeGB)
				assert.Equal(t, 300, *req.DiskSizeGB)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := buildModifyRequestFromConfig(tt.config, instance)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.checkReq != nil {
					tt.checkReq(t, req)
				}
			}
		})
	}
}
