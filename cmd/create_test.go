package cmd

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/pkg/types"
	"github.com/Thunder-Compute/thunder-cli/tui"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSpecStore() *utils.SpecStore {
	return utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"a100xl_x1_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 1, Mode: "production", VcpuOptions: []int{15}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"a100xl_x1_prototyping": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8, 12}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"a100xl_x2_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 2, Mode: "production", VcpuOptions: []int{30}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"a100xl_x2_prototyping": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 2, Mode: "prototyping", VcpuOptions: []int{8, 12, 16, 20, 24}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"a100xl_x4_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 4, Mode: "production", VcpuOptions: []int{60}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 2000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"a100xl_x8_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 8, Mode: "production", VcpuOptions: []int{120}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 4000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"a6000_x1_prototyping":  {DisplayName: "RTX A6000", VramGB: 48, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 6}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"h100_x1_production":    {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 1, Mode: "production", VcpuOptions: []int{15}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"h100_x1_prototyping":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8, 12, 16}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"h100_x2_production":    {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 2, Mode: "production", VcpuOptions: []int{30}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"h100_x2_prototyping":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 2, Mode: "prototyping", VcpuOptions: []int{8, 12, 16, 20, 24}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"h100_x4_production":    {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 4, Mode: "production", VcpuOptions: []int{60}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 2000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"h100_x8_production":    {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 8, Mode: "production", VcpuOptions: []int{120}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 4000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"l40_x1_production":     {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 1, Mode: "production", VcpuOptions: []int{10}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"l40_x1_prototyping":    {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"l40_x2_production":     {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 2, Mode: "production", VcpuOptions: []int{20}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"l40_x2_prototyping":    {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 2, Mode: "prototyping", VcpuOptions: []int{8, 12, 16}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"l40_x4_production":     {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 4, Mode: "production", VcpuOptions: []int{40}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 2000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
		"l40_x8_production":     {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 8, Mode: "production", VcpuOptions: []int{80}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 4000}, EphemeralStorageGB: api.StorageRange{Min: 0, Max: 0}},
	})
}

func tmplEntry(key, displayName string) api.TemplateEntry {
	return api.TemplateEntry{Key: key, Template: types.EnvironmentTemplate{DisplayName: displayName}}
}

func TestCreateFlagsDoNotRequireDisk(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("mode", "", "")
	cmd.Flags().String("gpu", "", "")
	cmd.Flags().String("template", "", "")
	cmd.Flags().String("snapshot", "", "")
	cmd.Flags().Int("num-gpus", 0, "")
	cmd.Flags().Int("vcpus", 0, "")
	cmd.Flags().Int("disk", 0, "")
	cmd.Flags().Int("disk-size-gb", 0, "")

	require.NoError(t, cmd.Flags().Set("mode", "production"))
	require.NoError(t, cmd.Flags().Set("gpu", "h100"))
	require.NoError(t, cmd.Flags().Set("template", "base"))
	require.NoError(t, cmd.Flags().Set("num-gpus", "4"))

	assert.Empty(t, missingCreateFlags(cmd))
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
				VCPUs:      6,
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
				VCPUs:      30,
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
			errorContains: "mode must be 'development' or 'production'",
		},
		{
			name: "invalid GPU type",
			config: &tui.CreateConfig{
				Mode:    "prototyping",
				GPUType: "invalid",
			},
			expectError:   true,
			errorContains: "development mode supports GPU types:",
		},
		{
			name: "prototyping without vcpus",
			config: &tui.CreateConfig{
				Mode:    "prototyping",
				GPUType: "a6000",
				VCPUs:   0,
			},
			expectError:   true,
			errorContains: "development mode requires --vcpus flag",
		},
		{
			name: "invalid vcpus for prototyping",
			config: &tui.CreateConfig{
				Mode:    "prototyping",
				GPUType: "a6000",
				VCPUs:   8,
			},
			expectError:   true,
			errorContains: "vcpus must be one of [4 6] for a6000 with 1 GPU(s)",
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
				VCPUs:      120,
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
				VCPUs:      6,
				Template:   "ubuntu-22.04",
				DiskSizeGB: 50,
			},
			templates: []api.TemplateEntry{
				tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
			},
			expectError:   true,
			errorContains: "disk size must be between 100 and 500 GB",
		},
		{
			name: "empty template is required error",
			config: &tui.CreateConfig{
				Mode:       "prototyping",
				GPUType:    "a6000",
				VCPUs:      6,
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
				VCPUs:      6,
				Template:   "nonexistent",
				DiskSizeGB: 100,
			},
			templates: []api.TemplateEntry{
				tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
			},
			expectError:   true,
			errorContains: "template or snapshot 'nonexistent' not found",
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
		VCPUs:      6,
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
	assert.Equal(t, 6, req.CPUCores)
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

	assert.Equal(t, 30, config.VCPUs)
}

func TestCreateConfigDefaultsDiskSizePerGPU(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:     "production",
		GPUType:  "h100",
		NumGPUs:  4,
		Template: "base",
	}

	templates := []api.TemplateEntry{
		tmplEntry("base", "Base"),
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore())
	require.NoError(t, err)

	assert.Equal(t, 400, config.DiskSizeGB)
}

func TestCreateConfigDefaultDiskSizeRespectsSnapshotMinimum(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:     "production",
		GPUType:  "h100",
		NumGPUs:  4,
		Template: "large-snapshot",
	}

	snapshots := []api.Snapshot{{
		Name:              "large-snapshot",
		MinimumDiskSizeGB: 550,
		Status:            "READY",
	}}

	err := validateCreateConfig(config, []api.TemplateEntry{}, snapshots, false, testSpecStore())
	require.NoError(t, err)

	assert.Equal(t, 550, config.DiskSizeGB)
}

func TestCreateConfigExplicitDiskCanBeBelowIncludedStorage(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "production",
		GPUType:    "h100",
		NumGPUs:    4,
		Template:   "base",
		DiskSizeGB: 100,
	}

	templates := []api.TemplateEntry{
		tmplEntry("base", "Base"),
	}

	err := validateCreateConfig(config, templates, []api.Snapshot{}, true, testSpecStore())
	require.NoError(t, err)

	assert.Equal(t, 100, config.DiskSizeGB)
}

// TestCreateConfigGPUTypeCaseInsensitive verifies that GPU type validation
// is case-insensitive and converts input to lowercase.
func TestCreateConfigGPUTypeCaseInsensitive(t *testing.T) {
	config := &tui.CreateConfig{
		Mode:       "prototyping",
		GPUType:    "A6000",
		VCPUs:      6,
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
		VCPUs:      6,
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
		VCPUs:      6,
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
// The a6000 prototyping spec has StorageGB: {Min: 100, Max: 500}.
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
			diskSizeGB:  500,
			expectError: false,
		},
		{
			name:        "disk size too small",
			diskSizeGB:  99,
			expectError: true,
		},
		{
			name:        "disk size too large",
			diskSizeGB:  501,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &tui.CreateConfig{
				Mode:       "prototyping",
				GPUType:    "a6000",
				VCPUs:      6,
				Template:   "ubuntu-22.04",
				DiskSizeGB: tt.diskSizeGB,
			}

			templates := []api.TemplateEntry{
				tmplEntry("ubuntu-22.04", "Ubuntu 22.04"),
			}

			err := validateCreateConfig(config, templates, []api.Snapshot{}, false, testSpecStore())

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "disk size must be between 100 and 500 GB")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
