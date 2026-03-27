package cmd

import (
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/pkg/types"
	"github.com/Thunder-Compute/thunder-cli/tui"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSpecStore() *utils.SpecStore {
	return utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1_prototyping":  {DisplayName: "RTX A6000", VramGB: 48, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 300}},
		"a100xl_x1_prototyping": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8, 12}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}},
		"a100xl_x2_prototyping": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 2, Mode: "prototyping", VcpuOptions: []int{8, 12, 16, 20, 24}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"h100_x1_prototyping":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8, 12, 16}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}},
		"h100_x2_prototyping":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 2, Mode: "prototyping", VcpuOptions: []int{8, 12, 16, 20, 24}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"a100xl_x1_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 1, Mode: "production", VcpuOptions: []int{18}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"a100xl_x2_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 2, Mode: "production", VcpuOptions: []int{36}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"a100xl_x4_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 4, Mode: "production", VcpuOptions: []int{72}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"a100xl_x8_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 8, Mode: "production", VcpuOptions: []int{144}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"h100_x1_production":    {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 1, Mode: "production", VcpuOptions: []int{18}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"h100_x2_production":    {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 2, Mode: "production", VcpuOptions: []int{36}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"h100_x4_production":    {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 4, Mode: "production", VcpuOptions: []int{72}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"h100_x8_production":    {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 8, Mode: "production", VcpuOptions: []int{144}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
	})
}

func tmplEntry(key, displayName string) api.TemplateEntry {
	return api.TemplateEntry{Key: key, Template: types.EnvironmentTemplate{DisplayName: displayName}}
}

// TestValidateCreateConfig provides comprehensive validation testing for instance
// creation configurations, covering both prototyping and production modes with
// various GPU types, CPU configurations, and template validations.
func TestValidateCreateConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        *tui.CreateConfig
		templates     []api.TemplateEntry
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
			templates: []api.TemplateEntry{
				tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
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
			templates: []api.TemplateEntry{
				tmplEntry("pytorch", "PyTorch"),
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
			errorContains: "prototyping mode supports GPU types:",
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
			errorContains: "vcpus must be one of [4 8] for a6000 with 1 GPU(s)",
		},
		{
			name: "production with invalid GPU type",
			config: &tui.CreateConfig{
				Mode:    "production",
				GPUType: "a6000",
			},
			expectError:   true,
			errorContains: "production mode supports GPU types:",
		},
		{
			name: "production with invalid num-gpus",
			config: &tui.CreateConfig{
				Mode:       "production",
				GPUType:    "a100",
				NumGPUs:    3,
				Template:   "base",
				DiskSizeGB: 100,
			},
			templates: []api.TemplateEntry{
				tmplEntry("base", "Base ML Environment"),
			},
			expectError:   true,
			errorContains: "GPU count 3 is not valid",
		},
		{
			name: "invalid num-gpus for production",
			config: &tui.CreateConfig{
				Mode:    "production",
				GPUType: "a100",
				NumGPUs: 3,
			},
			expectError:   true,
			errorContains: "GPU count 3 is not valid",
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
			templates: []api.TemplateEntry{
				tmplEntry("pytorch", "PyTorch"),
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
			templates: []api.TemplateEntry{
				tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
			},
			expectError:   true,
			errorContains: "disk size must be between 100 and 300 GB",
		},
		{
			name: "empty template is required error",
			config: &tui.CreateConfig{
				Mode:       "prototyping",
				GPUType:    "a6000",
				VCPUs:      8,
				DiskSizeGB: 100,
			},
			templates: []api.TemplateEntry{
				tmplEntry("base", "Base ML Environment"),
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
			templates: []api.TemplateEntry{
				tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
			},
			expectError:   true,
			errorContains: "template 'nonexistent' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateConfig(tt.config, tt.templates, []api.Snapshot{}, false, testSpecStore())

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

	templates := []api.TemplateEntry{
		tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
	}

	require.NoError(t, validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore()))

	req := api.CreateInstanceRequest{
		Mode:       api.InstanceMode(config.Mode),
		GPUType:    config.GPUType,
		NumGPUs:    config.NumGPUs,
		CPUCores:   config.VCPUs,
		Template:   config.Template,
		DiskSizeGB: config.DiskSizeGB,
	}

	assert.Equal(t, api.InstanceMode("prototyping"), req.Mode)
	assert.Equal(t, "a6000", req.GPUType)
	assert.Equal(t, 1, req.NumGPUs)
	assert.Equal(t, 8, req.CPUCores)
	assert.Equal(t, "ubuntu-22.04", req.Template)
	assert.Equal(t, 100, req.DiskSizeGB)
}

func TestCreateInstanceRequestA100Alias(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "prototyping",
		GPUType:    "a100",
		VCPUs:      8,
		Template:   "ubuntu-22.04",
		DiskSizeGB: 100,
	}

	templates := []api.TemplateEntry{
		tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
	}

	require.NoError(t, validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore()))

	req := api.CreateInstanceRequest{
		Mode:       api.InstanceMode(config.Mode),
		GPUType:    config.GPUType,
		NumGPUs:    config.NumGPUs,
		CPUCores:   config.VCPUs,
		Template:   config.Template,
		DiskSizeGB: config.DiskSizeGB,
	}

	assert.Equal(t, api.InstanceMode("prototyping"), req.Mode)
	assert.Equal(t, "a100xl", req.GPUType)
	assert.Equal(t, 1, req.NumGPUs)
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

	templates := []api.TemplateEntry{
		tmplEntry("pytorch", "PyTorch"),
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore())
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

	templates := []api.TemplateEntry{
		tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore())
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

	templates := []api.TemplateEntry{
		tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore())
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

	templates := []api.TemplateEntry{
		tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore())
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

	templates := []api.TemplateEntry{
		tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore())
	require.NoError(t, err)

	assert.Equal(t, "ubuntu-22.04", config.Template)
}

// TestCreateConfigDiskSizeBoundaries verifies that disk size validation
// correctly enforces the storage range from the GPU spec.
// The a6000 prototyping spec has StorageGB: {Min: 100, Max: 300}.
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
			diskSizeGB:  300,
			expectError: false,
		},
		{
			name:        "disk size too small",
			diskSizeGB:  99,
			expectError: true,
		},
		{
			name:        "disk size too large",
			diskSizeGB:  301,
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

			templates := []api.TemplateEntry{
				tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
			}

			err := validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore())

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "disk size must be between 100 and 300 GB")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
