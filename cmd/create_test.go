package cmd

import (
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateCreateConfig provides comprehensive validation testing for instance
// creation configurations, covering both prototyping and production modes with
// various GPU types, CPU configurations, and template validations.
func TestValidateCreateConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        *tui.CreateConfig
		templates     []api.Template
		expectError   bool
		errorContains string
	}{
		{
			name: "valid prototyping config",
			config: &tui.CreateConfig{
				Mode:       "prototyping",
				GPUType:    "a6000",
				NumGPUs:    1,
				VCPUs:      8,
				Template:   "ubuntu-22.04",
				DiskSizeGB: 100,
			},
			templates: []api.Template{
				{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
			},
			expectError: false,
		},
		{
			name: "valid production config",
			config: &tui.CreateConfig{
				Mode:       "production",
				GPUType:    "a100",
				NumGPUs:    2,
				VCPUs:      36,
				Template:   "pytorch",
				DiskSizeGB: 500,
			},
			templates: []api.Template{
				{Key: "pytorch", DisplayName: "PyTorch"},
			},
			expectError: false,
		},
		{
			name: "invalid mode",
			config: &tui.CreateConfig{
				Mode: "invalid",
			},
			expectError:   true,
			errorContains: "mode must be 'prototyping' or 'production'",
		},
		{
			name: "invalid GPU type",
			config: &tui.CreateConfig{
				Mode:    "prototyping",
				GPUType: "invalid",
			},
			expectError:   true,
			errorContains: "prototyping mode supports GPU types: a6000 or a100",
		},
		{
			name: "prototyping without vcpus",
			config: &tui.CreateConfig{
				Mode:    "prototyping",
				GPUType: "a6000",
				VCPUs:   0,
			},
			expectError:   true,
			errorContains: "prototyping mode requires --vcpus flag",
		},
		{
			name: "invalid vcpus for prototyping",
			config: &tui.CreateConfig{
				Mode:    "prototyping",
				GPUType: "a6000",
				VCPUs:   6,
			},
			expectError:   true,
			errorContains: "vcpus must be one of: 4, 8, 16, or 32",
		},
		{
			name: "production with invalid GPU type",
			config: &tui.CreateConfig{
				Mode:    "production",
				GPUType: "a6000",
			},
			expectError:   true,
			errorContains: "production mode supports GPU types: a100 or h100",
		},
		{
			name: "production without num-gpus",
			config: &tui.CreateConfig{
				Mode:    "production",
				GPUType: "a100",
				NumGPUs: 0,
			},
			expectError:   true,
			errorContains: "production mode requires --num-gpus flag",
		},
		{
			name: "invalid num-gpus for production",
			config: &tui.CreateConfig{
				Mode:    "production",
				GPUType: "a100",
				NumGPUs: 3,
			},
			expectError:   true,
			errorContains: "num-gpus must be one of: 1, 2, 4, or 8",
		},
		{
			name: "valid production config with 8 GPUs",
			config: &tui.CreateConfig{
				Mode:       "production",
				GPUType:    "a100",
				NumGPUs:    8,
				VCPUs:      144,
				Template:   "pytorch",
				DiskSizeGB: 500,
			},
			templates: []api.Template{
				{Key: "pytorch", DisplayName: "PyTorch"},
			},
			expectError: false,
		},
		{
			name: "invalid disk size",
			config: &tui.CreateConfig{
				Mode:       "prototyping",
				GPUType:    "a6000",
				VCPUs:      8,
				Template:   "ubuntu-22.04",
				DiskSizeGB: 50,
			},
			templates: []api.Template{
				{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
			},
			expectError:   true,
			errorContains: "disk size must be between 100 and 1000 GB",
		},
		{
			name: "missing template",
			config: &tui.CreateConfig{
				Mode:       "prototyping",
				GPUType:    "a6000",
				VCPUs:      8,
				DiskSizeGB: 100,
			},
			expectError:   true,
			errorContains: "template is required",
		},
		{
			name: "template not found",
			config: &tui.CreateConfig{
				Mode:       "prototyping",
				GPUType:    "a6000",
				VCPUs:      8,
				Template:   "nonexistent",
				DiskSizeGB: 100,
			},
			templates: []api.Template{
				{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
			},
			expectError:   true,
			errorContains: "template 'nonexistent' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateConfig(tt.config, tt.templates, []api.Snapshot{}, false)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateInstanceRequest(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "prototyping",
		GPUType:    "a6000",
		NumGPUs:    1,
		VCPUs:      8,
		Template:   "ubuntu-22.04",
		DiskSizeGB: 100,
	}

	templates := []api.Template{
		{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
	}

	require.NoError(t, validateCreateConfig(config, templates, []api.Snapshot{}, false))

	req := api.CreateInstanceRequest{
		Mode:       api.InstanceMode(config.Mode),
		GpuType:    config.GPUType,
		NumGpus:    config.NumGPUs,
		CpuCores:   config.VCPUs,
		Template:   config.Template,
		DiskSizeGb: config.DiskSizeGB,
	}

	assert.Equal(t, api.InstanceMode("prototyping"), req.Mode)
	assert.Equal(t, "a6000", req.GpuType)
	assert.Equal(t, 1, req.NumGpus)
	assert.Equal(t, 8, req.CpuCores)
	assert.Equal(t, "ubuntu-22.04", req.Template)
	assert.Equal(t, 100, req.DiskSizeGb)
}

func TestCreateInstanceRequestA100Alias(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "prototyping",
		GPUType:    "a100",
		VCPUs:      8,
		Template:   "ubuntu-22.04",
		DiskSizeGB: 100,
	}

	templates := []api.Template{
		{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
	}

	require.NoError(t, validateCreateConfig(config, templates, []api.Snapshot{}, false))

	req := api.CreateInstanceRequest{
		Mode:       api.InstanceMode(config.Mode),
		GpuType:    config.GPUType,
		NumGpus:    config.NumGPUs,
		CpuCores:   config.VCPUs,
		Template:   config.Template,
		DiskSizeGb: config.DiskSizeGB,
	}

	assert.Equal(t, api.InstanceMode("prototyping"), req.Mode)
	assert.Equal(t, "a100xl", req.GpuType)
	assert.Equal(t, 1, req.NumGpus)
}

// TestCreateConfigVCPUsAutoSet verifies that VCPUs are automatically calculated
// for production mode instances based on the number of GPUs.
func TestCreateConfigVCPUsAutoSet(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "production",
		GPUType:    "a100",
		NumGPUs:    2,
		VCPUs:      0,
		Template:   "pytorch",
		DiskSizeGB: 100,
	}

	templates := []api.Template{
		{Key: "pytorch", DisplayName: "PyTorch"},
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false)
	require.NoError(t, err)

	assert.Equal(t, 36, config.VCPUs)
}

// TestCreateConfigGPUTypeCaseInsensitive verifies that GPU type validation
// is case-insensitive and converts input to lowercase.
func TestCreateConfigGPUTypeCaseInsensitive(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "prototyping",
		GPUType:    "A6000",
		VCPUs:      8,
		Template:   "ubuntu-22.04",
		DiskSizeGB: 100,
	}

	templates := []api.Template{
		{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false)
	require.NoError(t, err)

	assert.Equal(t, "a6000", config.GPUType)
}

func TestCreateConfigA100Alias(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "prototyping",
		GPUType:    "A100",
		VCPUs:      8,
		Template:   "ubuntu-22.04",
		DiskSizeGB: 100,
	}

	templates := []api.Template{
		{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false)
	require.NoError(t, err)

	assert.Equal(t, "a100xl", config.GPUType)
}

// TestCreateConfigTemplateCaseInsensitive verifies that template matching
// is case-insensitive when comparing with display names.
func TestCreateConfigTemplateCaseInsensitive(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "prototyping",
		GPUType:    "a6000",
		VCPUs:      8,
		Template:   "UBUNTU 22.04",
		DiskSizeGB: 100,
	}

	templates := []api.Template{
		{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false)
	require.NoError(t, err)

	assert.Equal(t, "ubuntu-22.04", config.Template)
}

// TestCreateConfigTemplateByDisplayName verifies that templates can be
// matched by their display name and converted to the appropriate key.
func TestCreateConfigTemplateByDisplayName(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "prototyping",
		GPUType:    "a6000",
		VCPUs:      8,
		Template:   "Ubuntu 22.04",
		DiskSizeGB: 100,
	}

	templates := []api.Template{
		{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false)
	require.NoError(t, err)

	assert.Equal(t, "ubuntu-22.04", config.Template)
}

// TestCreateConfigDiskSizeBoundaries verifies that disk size validation
// correctly enforces the 100-1000 GB range for instance creation.
func TestCreateConfigDiskSizeBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		diskSizeGB  int
		expectError bool
	}{
		{
			name:        "minimum valid disk size",
			diskSizeGB:  100,
			expectError: false,
		},
		{
			name:        "maximum valid disk size",
			diskSizeGB:  1000,
			expectError: false,
		},
		{
			name:        "disk size too small",
			diskSizeGB:  99,
			expectError: true,
		},
		{
			name:        "disk size too large",
			diskSizeGB:  1001,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &tui.CreateConfig{
				Mode:       "prototyping",
				GPUType:    "a6000",
				VCPUs:      8,
				Template:   "ubuntu-22.04",
				DiskSizeGB: tt.diskSizeGB,
			}

			templates := []api.Template{
				{Key: "ubuntu-22.04", DisplayName: "Ubuntu 22.04"},
			}

			err := validateCreateConfig(config, templates, []api.Snapshot{}, false)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "disk size must be between 100 and 1000 GB")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
