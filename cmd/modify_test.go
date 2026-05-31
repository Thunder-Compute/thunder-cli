package cmd

import (
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func modifySpecStore() *utils.SpecStore {
	return testSpecStore()
}

func modifyInstance(mode, gpuType, numGPUs, cpuCores string, storage int) *api.Instance {
	return &api.Instance{
		ID:       "test-1",
		UUID:     "uuid-1",
		Name:     "test-instance",
		Status:   "RUNNING",
		Mode:     mode,
		GPUType:  gpuType,
		NumGPUs:  numGPUs,
		CPUCores: cpuCores,
		Storage:  storage,
	}
}

// modifyCmd creates a fresh cobra command with modify flags for testing.
func newModifyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "modify"}
	cmd.Flags().String("mode", "", "")
	cmd.Flags().String("gpu", "", "")
	cmd.Flags().Int("num-gpus", 0, "")
	cmd.Flags().Int("vcpus", 0, "")
	cmd.Flags().Int("primary-disk", 0, "")
	return cmd
}

func setFlags(cmd *cobra.Command, flags map[string]string) {
	for k, v := range flags {
		_ = cmd.Flags().Set(k, v)
	}
}

// ── buildModifyRequestFromFlags ─────────────────────────────────────────────

func TestBuildModifyRequestFromFlags(t *testing.T) {
	specs := modifySpecStore()

	tests := []struct {
		name          string
		instance      *api.Instance
		flags         map[string]string
		expectError   bool
		errorContains string
		validate      func(t *testing.T, req api.InstanceModifyRequest)
	}{
		{
			name:     "change GPU type within prototyping",
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			flags:    map[string]string{"gpu": "h100"},
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.GPUType)
				assert.Equal(t, "h100", *req.GPUType)
				assert.Nil(t, req.Mode)
			},
		},
		{
			name:     "change disk size",
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			flags:    map[string]string{"primary-disk": "200"},
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.DiskSizeGB)
				assert.Equal(t, 200, *req.DiskSizeGB)
			},
		},
		{
			name:          "disk shrink rejected",
			instance:      modifyInstance("prototyping", "a6000", "1", "8", 200),
			flags:         map[string]string{"primary-disk": "150"},
			expectError:   true,
			errorContains: "cannot be smaller than current size (200 GB)",
		},
		{
			name:          "disk exceeds max",
			instance:      modifyInstance("prototyping", "a6000", "1", "8", 100),
			flags:         map[string]string{"primary-disk": "500"},
			expectError:   true,
			errorContains: "disk size must be between",
		},
		{
			name:          "invalid mode",
			instance:      modifyInstance("prototyping", "a6000", "1", "8", 100),
			flags:         map[string]string{"mode": "invalid"},
			expectError:   true,
			errorContains: "mode must be 'prototyping' or 'production'",
		},
		{
			name:          "switch to production without num-gpus",
			instance:      modifyInstance("prototyping", "a6000", "1", "8", 100),
			flags:         map[string]string{"mode": "production"},
			expectError:   true,
			errorContains: "switching to production requires --num-gpus",
		},
		{
			name:          "switch to prototyping without vcpus",
			instance:      modifyInstance("production", "a100xl", "1", "18", 100),
			flags:         map[string]string{"mode": "prototyping"},
			expectError:   true,
			errorContains: "switching to prototyping requires --vcpus",
		},
		{
			name:     "switch to production with num-gpus",
			instance: modifyInstance("prototyping", "h100", "1", "8", 100),
			flags:    map[string]string{"mode": "production", "num-gpus": "1"},
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.Mode)
				assert.Equal(t, api.InstanceMode("production"), *req.Mode)
				require.NotNil(t, req.NumGPUs)
				assert.Equal(t, 1, *req.NumGPUs)
			},
		},
		{
			name:          "vcpus in production mode rejected",
			instance:      modifyInstance("production", "a100xl", "1", "18", 100),
			flags:         map[string]string{"vcpus": "8"},
			expectError:   true,
			errorContains: "production mode does not use --vcpus flag",
		},
		{
			name:          "invalid vcpus for GPU",
			instance:      modifyInstance("prototyping", "a6000", "1", "4", 100),
			flags:         map[string]string{"vcpus": "16"},
			expectError:   true,
			errorContains: "vcpus must be one of",
		},
		{
			name:     "valid vcpus for GPU",
			instance: modifyInstance("prototyping", "a6000", "1", "4", 100),
			flags:    map[string]string{"vcpus": "8"},
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.CPUCores)
				assert.Equal(t, 8, *req.CPUCores)
			},
		},
		{
			name:          "invalid GPU type for mode",
			instance:      modifyInstance("production", "a100xl", "1", "18", 100),
			flags:         map[string]string{"gpu": "a6000"},
			expectError:   true,
			errorContains: "invalid GPU type",
		},
		{
			name:          "invalid num-gpus count",
			instance:      modifyInstance("production", "a100xl", "1", "18", 100),
			flags:         map[string]string{"num-gpus": "3"},
			expectError:   true,
			errorContains: "num-gpus 3 is not valid",
		},
		{
			name:     "valid num-gpus change",
			instance: modifyInstance("production", "a100xl", "1", "18", 100),
			flags:    map[string]string{"num-gpus": "2"},
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.NumGPUs)
				assert.Equal(t, 2, *req.NumGPUs)
				require.NotNil(t, req.CPUCores)
				assert.Equal(t, 36, *req.CPUCores)
			},
		},
		{
			name:          "no flags set returns error",
			instance:      modifyInstance("prototyping", "a6000", "1", "8", 100),
			flags:         map[string]string{},
			expectError:   true,
			errorContains: "no changes specified",
		},
		{
			name:     "gpu type case insensitive",
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			flags:    map[string]string{"gpu": "H100"},
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.GPUType)
				assert.Equal(t, "h100", *req.GPUType)
			},
		},
		{
			name:     "same mode does not require dependent flags",
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			flags:    map[string]string{"mode": "prototyping"},
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.Mode)
				assert.Equal(t, api.InstanceMode("prototyping"), *req.Mode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newModifyCmd()
			setFlags(cmd, tt.flags)

			req, err := buildModifyRequestFromFlags(cmd, tt.instance, specs)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, req)
				}
			}
		})
	}
}

func TestBuildModifyRequestFromFlags_ValidatesVCPUsAgainstRequestedGPUCount(t *testing.T) {
	specs := utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1_prototyping": {
			GpuCount:           1,
			Mode:               "prototyping",
			VcpuOptions:        []int{4, 8},
			StorageGB:          api.StorageRange{Min: 100, Max: 300},
			EphemeralStorageGB: api.StorageRange{Min: 0, Max: 2000},
		},
		"a6000_x4_prototyping": {
			GpuCount:           4,
			Mode:               "prototyping",
			VcpuOptions:        []int{16, 24, 32},
			StorageGB:          api.StorageRange{Min: 100, Max: 1000},
			EphemeralStorageGB: api.StorageRange{Min: 0, Max: 2000},
		},
	})
	cmd := newModifyCmd()
	setFlags(cmd, map[string]string{"num-gpus": "4", "vcpus": "16"})

	req, err := buildModifyRequestFromFlags(cmd, modifyInstance("prototyping", "a6000", "1", "8", 100), specs)

	require.NoError(t, err)
	require.NotNil(t, req.NumGPUs)
	assert.Equal(t, 4, *req.NumGPUs)
	require.NotNil(t, req.CPUCores)
	assert.Equal(t, 16, *req.CPUCores)
}

func TestBuildModifyRequestFromFlags_RejectsInvalidTargetSpecValues(t *testing.T) {
	specs := utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1_prototyping": {
			GpuCount:           1,
			Mode:               "prototyping",
			VcpuOptions:        []int{4, 8},
			StorageGB:          api.StorageRange{Min: 100, Max: 300},
			EphemeralStorageGB: api.StorageRange{Min: 0, Max: 500},
		},
		"a6000_x4_prototyping": {
			GpuCount:           4,
			Mode:               "prototyping",
			VcpuOptions:        []int{16, 24, 32},
			StorageGB:          api.StorageRange{Min: 200, Max: 1000},
			EphemeralStorageGB: api.StorageRange{Min: 0, Max: 2000},
		},
	})

	tests := []struct {
		name          string
		flags         map[string]string
		errorContains string
	}{
		{
			name:          "gpu count change requires vcpu valid for target count",
			flags:         map[string]string{"num-gpus": "4"},
			errorContains: "vcpus must be one of [16 24 32]",
		},
		{
			name:          "primary disk validates target spec minimum",
			flags:         map[string]string{"num-gpus": "4", "vcpus": "16", "primary-disk": "150"},
			errorContains: "disk size must be between 200 and 1000 GB",
		},
		{
			name:          "ephemeral disk validates target spec range",
			flags:         map[string]string{"ephemeral-disk": "600"},
			errorContains: "ephemeral disk size must be between 0 and 500 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newModifyCmd()
			cmd.Flags().Int("ephemeral-disk", -1, "")
			setFlags(cmd, tt.flags)

			_, err := buildModifyRequestFromFlags(cmd, modifyInstance("prototyping", "a6000", "1", "8", 100), specs)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

func TestBuildModifyRequestFromFlags_RejectsUnavailableTargetSpec(t *testing.T) {
	specs := utils.NewSpecStoreWithAvailability(map[string]api.GpuSpecConfig{
		"a6000_x1_prototyping": {
			GpuCount:           1,
			Mode:               "prototyping",
			VcpuOptions:        []int{4, 8},
			StorageGB:          api.StorageRange{Min: 100, Max: 300},
			EphemeralStorageGB: api.StorageRange{Min: 0, Max: 500},
		},
	}, map[string]string{
		"a6000_x1_prototyping": "unavailable",
	})

	cmd := newModifyCmd()
	setFlags(cmd, map[string]string{"primary-disk": "150"})

	_, err := buildModifyRequestFromFlags(cmd, modifyInstance("prototyping", "a6000", "1", "8", 100), specs)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GPU configuration a6000 x1 in prototyping mode is currently unavailable")
}

// ── buildModifyRequestFromConfig ────────────────────────────────────────────

func TestBuildModifyRequestFromConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        *tui.ModifyConfig
		instance      *api.Instance
		expectError   bool
		errorContains string
		validate      func(t *testing.T, req api.InstanceModifyRequest)
	}{
		{
			name: "mode change only",
			config: &tui.ModifyConfig{
				Mode:        "production",
				ModeChanged: true,
			},
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.Mode)
				assert.Equal(t, api.InstanceMode("production"), *req.Mode)
				assert.Nil(t, req.GPUType)
				assert.Nil(t, req.DiskSizeGB)
			},
		},
		{
			name: "GPU change only",
			config: &tui.ModifyConfig{
				GPUType:    "h100",
				GPUChanged: true,
			},
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.GPUType)
				assert.Equal(t, "h100", *req.GPUType)
				assert.Nil(t, req.Mode)
			},
		},
		{
			name: "compute change in prototyping sets vcpus",
			config: &tui.ModifyConfig{
				VCPUs:          12,
				ComputeChanged: true,
			},
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.CPUCores)
				assert.Equal(t, 12, *req.CPUCores)
				assert.Nil(t, req.NumGPUs)
			},
		},
		{
			name: "compute change in prototyping sets num-gpus",
			config: &tui.ModifyConfig{
				NumGPUs:        4,
				VCPUs:          8,
				ComputeChanged: true,
			},
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.NumGPUs)
				assert.Equal(t, 4, *req.NumGPUs)
				assert.Nil(t, req.CPUCores)
			},
		},
		{
			name: "compute change in prototyping sets num-gpus and vcpus",
			config: &tui.ModifyConfig{
				NumGPUs:        4,
				VCPUs:          16,
				ComputeChanged: true,
			},
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.NumGPUs)
				assert.Equal(t, 4, *req.NumGPUs)
				require.NotNil(t, req.CPUCores)
				assert.Equal(t, 16, *req.CPUCores)
			},
		},
		{
			name: "compute change in production sets num-gpus",
			config: &tui.ModifyConfig{
				NumGPUs:        2,
				VCPUs:          36,
				ComputeChanged: true,
			},
			instance: modifyInstance("production", "a100xl", "1", "18", 100),
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.NumGPUs)
				assert.Equal(t, 2, *req.NumGPUs)
				require.NotNil(t, req.CPUCores)
				assert.Equal(t, 36, *req.CPUCores)
			},
		},
		{
			name: "compute change with mode switch uses new mode",
			config: &tui.ModifyConfig{
				Mode:           "production",
				ModeChanged:    true,
				NumGPUs:        4,
				ComputeChanged: true,
			},
			instance: modifyInstance("prototyping", "h100", "1", "8", 100),
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.Mode)
				require.NotNil(t, req.NumGPUs)
				assert.Equal(t, 4, *req.NumGPUs)
				assert.Nil(t, req.CPUCores)
			},
		},
		{
			name: "disk change",
			config: &tui.ModifyConfig{
				DiskSizeGB:  300,
				DiskChanged: true,
			},
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.DiskSizeGB)
				assert.Equal(t, 300, *req.DiskSizeGB)
			},
		},
		{
			name: "all changes at once",
			config: &tui.ModifyConfig{
				Mode:           "production",
				ModeChanged:    true,
				GPUType:        "h100",
				GPUChanged:     true,
				NumGPUs:        2,
				ComputeChanged: true,
				DiskSizeGB:     500,
				DiskChanged:    true,
			},
			instance: modifyInstance("prototyping", "a6000", "1", "8", 100),
			validate: func(t *testing.T, req api.InstanceModifyRequest) {
				require.NotNil(t, req.Mode)
				require.NotNil(t, req.GPUType)
				require.NotNil(t, req.NumGPUs)
				require.NotNil(t, req.DiskSizeGB)
			},
		},
		{
			name: "no changes returns error",
			config: &tui.ModifyConfig{
				ModeChanged:    false,
				GPUChanged:     false,
				ComputeChanged: false,
				DiskChanged:    false,
			},
			instance:      modifyInstance("prototyping", "a6000", "1", "8", 100),
			expectError:   true,
			errorContains: "no changes specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := buildModifyRequestFromConfig(tt.config, tt.instance)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, req)
				}
			}
		})
	}
}

// ── hasAllModifyFlags ───────────────────────────────────────────────────────

func TestHasAllModifyFlags(t *testing.T) {
	t.Run("no flags returns false", func(t *testing.T) {
		cmd := newModifyCmd()
		assert.False(t, hasAllModifyFlags(cmd))
	})

	t.Run("single flag returns true", func(t *testing.T) {
		cmd := newModifyCmd()
		_ = cmd.Flags().Set("mode", "production")
		assert.True(t, hasAllModifyFlags(cmd))
	})

	t.Run("multiple flags returns true", func(t *testing.T) {
		cmd := newModifyCmd()
		_ = cmd.Flags().Set("mode", "production")
		_ = cmd.Flags().Set("gpu", "h100")
		assert.True(t, hasAllModifyFlags(cmd))
	})
}

// ── buildModifyPresets ──────────────────────────────────────────────────────

func TestBuildModifyPresets(t *testing.T) {
	t.Run("no flags set produces empty presets", func(t *testing.T) {
		cmd := newModifyCmd()
		p := buildModifyPresets(cmd)
		assert.True(t, p.IsEmpty())
	})

	t.Run("all flags set populates all fields", func(t *testing.T) {
		cmd := newModifyCmd()
		_ = cmd.Flags().Set("mode", "production")
		_ = cmd.Flags().Set("gpu", "h100")
		_ = cmd.Flags().Set("num-gpus", "2")
		_ = cmd.Flags().Set("vcpus", "8")
		_ = cmd.Flags().Set("primary-disk", "200")

		p := buildModifyPresets(cmd)
		assert.False(t, p.IsEmpty())
		require.NotNil(t, p.Mode)
		assert.Equal(t, "production", *p.Mode)
		require.NotNil(t, p.GPUType)
		assert.Equal(t, "h100", *p.GPUType)
		require.NotNil(t, p.NumGPUs)
		assert.Equal(t, 2, *p.NumGPUs)
		require.NotNil(t, p.VCPUs)
		assert.Equal(t, 8, *p.VCPUs)
		require.NotNil(t, p.DiskSizeGB)
		assert.Equal(t, 200, *p.DiskSizeGB)
	})

	t.Run("partial flags set", func(t *testing.T) {
		cmd := newModifyCmd()
		_ = cmd.Flags().Set("gpu", "a100")

		p := buildModifyPresets(cmd)
		assert.False(t, p.IsEmpty())
		assert.Nil(t, p.Mode)
		require.NotNil(t, p.GPUType)
		assert.Equal(t, "a100", *p.GPUType)
	})
}
