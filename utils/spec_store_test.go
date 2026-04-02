package utils

import (
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/stretchr/testify/assert"
)

func testSpecStore() *SpecStore {
	return NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1_prototyping":  {DisplayName: "RTX A6000", VramGB: 48, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 300}},
		"a100xl_x1_prototyping": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8, 12}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}},
		"a100xl_x2_prototyping": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 2, Mode: "prototyping", VcpuOptions: []int{8, 12, 16, 20, 24}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"h100_x1_prototyping":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8, 12, 16}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}},
		"h100_x2_prototyping":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 2, Mode: "prototyping", VcpuOptions: []int{8, 12, 16, 20, 24}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"a100xl_x1_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 1, Mode: "production", VcpuOptions: []int{18}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"a100xl_x2_production":  {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 2, Mode: "production", VcpuOptions: []int{36}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"h100_x1_production":    {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 1, Mode: "production", VcpuOptions: []int{18}, RamPerVCPUGiB: 5, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
	})
}

func TestGPUOptionsForMode(t *testing.T) {
	s := testSpecStore()

	tests := []struct {
		name     string
		mode     string
		expected []string
	}{
		{
			name:     "prototyping includes a6000, a100xl, h100 in order",
			mode:     "prototyping",
			expected: []string{"a6000", "a100xl", "h100"},
		},
		{
			name:     "production excludes a6000",
			mode:     "production",
			expected: []string{"a100xl", "h100"},
		},
		{
			name:     "unknown mode returns empty",
			mode:     "unknown",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.GPUOptionsForMode(tt.mode)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGPUOptionsForMode_UnknownGPUAppended(t *testing.T) {
	s := NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1_prototyping": {GpuCount: 1, Mode: "prototyping"},
		"b200_x1_prototyping":  {GpuCount: 1, Mode: "prototyping"},
	})

	got := s.GPUOptionsForMode("prototyping")
	// a6000 should come first (in display order), b200 appended after
	assert.Equal(t, "a6000", got[0])
	assert.Contains(t, got, "b200")
}

func TestGPUCountsForMode(t *testing.T) {
	s := testSpecStore()

	tests := []struct {
		name     string
		gpuType  string
		mode     string
		expected []int
	}{
		{
			name:     "a6000 prototyping only has 1 GPU",
			gpuType:  "a6000",
			mode:     "prototyping",
			expected: []int{1},
		},
		{
			name:     "a100xl prototyping has 1 and 2 GPUs",
			gpuType:  "a100xl",
			mode:     "prototyping",
			expected: []int{1, 2},
		},
		{
			name:     "a100xl production has 1 and 2 GPUs",
			gpuType:  "a100xl",
			mode:     "production",
			expected: []int{1, 2},
		},
		{
			name:     "unknown GPU returns nil",
			gpuType:  "unknown",
			mode:     "prototyping",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.GPUCountsForMode(tt.gpuType, tt.mode)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNeedsGPUCountPhase(t *testing.T) {
	s := testSpecStore()

	assert.False(t, s.NeedsGPUCountPhase("a6000", "prototyping"), "single-count GPU should not need count phase")
	assert.True(t, s.NeedsGPUCountPhase("a100xl", "prototyping"), "multi-count GPU should need count phase")
	assert.True(t, s.NeedsGPUCountPhase("h100", "prototyping"), "h100 has x1 and x2")
}

func TestVCPUOptions(t *testing.T) {
	s := testSpecStore()

	assert.Equal(t, []int{4, 8}, s.VCPUOptions("a6000", 1, "prototyping"))
	assert.Equal(t, []int{18}, s.VCPUOptions("a100xl", 1, "production"))
	assert.Nil(t, s.VCPUOptions("unknown", 1, "prototyping"))
}

func TestIncludedVCPUs(t *testing.T) {
	s := testSpecStore()

	assert.Equal(t, 4, s.IncludedVCPUs("a6000", 1, "prototyping"))
	assert.Equal(t, 8, s.IncludedVCPUs("a100xl", 2, "prototyping"))
	assert.Equal(t, 4, s.IncludedVCPUs("unknown", 1, "prototyping"), "fallback to 4")
}

func TestRamPerVCPU(t *testing.T) {
	s := testSpecStore()

	assert.Equal(t, 8, s.RamPerVCPU("a6000", 1, "prototyping"))
	assert.Equal(t, 5, s.RamPerVCPU("a100xl", 1, "production"))
	assert.Equal(t, 8, s.RamPerVCPU("unknown", 1, "prototyping"), "prototyping fallback")
	assert.Equal(t, 5, s.RamPerVCPU("unknown", 1, "production"), "production fallback")
}

func TestStorageRange(t *testing.T) {
	s := testSpecStore()

	tests := []struct {
		name           string
		gpuType        string
		numGPUs        int
		mode           string
		expectedMin    int
		expectedMax    int
	}{
		{"a6000 prototyping", "a6000", 1, "prototyping", 100, 300},
		{"a100xl x1 prototyping", "a100xl", 1, "prototyping", 100, 500},
		{"a100xl x2 prototyping", "a100xl", 2, "prototyping", 100, 1000},
		{"production", "a100xl", 1, "production", 100, 1000},
		{"unknown falls back to 100-1000", "unknown", 1, "prototyping", 100, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minGB, maxGB := s.StorageRange(tt.gpuType, tt.numGPUs, tt.mode)
			assert.Equal(t, tt.expectedMin, minGB)
			assert.Equal(t, tt.expectedMax, maxGB)
		})
	}
}


func TestNormalizeGPUType(t *testing.T) {
	s := testSpecStore()

	tests := []struct {
		name     string
		input    string
		mode     string
		wantGPU  string
		wantOK   bool
	}{
		{"exact match", "a6000", "prototyping", "a6000", true},
		{"a100 alias", "a100", "prototyping", "a100xl", true},
		{"uppercase normalized", "A100", "prototyping", "a100xl", true},
		{"a6000 not in production", "a6000", "production", "a6000", false},
		{"unknown GPU", "v100", "prototyping", "v100", false},
		{"h100 in production", "h100", "production", "h100", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpu, ok := s.NormalizeGPUType(tt.input, tt.mode)
			assert.Equal(t, tt.wantGPU, gpu)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestLookup(t *testing.T) {
	s := testSpecStore()

	spec := s.Lookup("a6000", 1, "prototyping")
	assert.NotNil(t, spec)
	assert.Equal(t, "RTX A6000", spec.DisplayName)
	assert.Equal(t, 48, spec.VramGB)

	assert.Nil(t, s.Lookup("unknown", 1, "prototyping"))
	assert.Nil(t, s.Lookup("a6000", 1, "production"))
}
