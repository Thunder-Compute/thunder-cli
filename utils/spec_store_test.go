package utils

import (
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/stretchr/testify/assert"
)

func testSpecStore() *SpecStore {
	return NewSpecStore(map[string]api.GpuSpecConfig{
		"a100xl_x1": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 1, VcpuOptions: []int{4, 8, 12}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}},
		"a100xl_x2": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 2, VcpuOptions: []int{8, 12, 16, 20, 24}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"a100xl_x4": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 4, VcpuOptions: []int{60}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 2000}},
		"a100xl_x8": {DisplayName: "NVIDIA A100 (80GB)", VramGB: 80, GpuCount: 8, VcpuOptions: []int{120}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 4000}},
		"a6000_x1":  {DisplayName: "RTX A6000", VramGB: 48, GpuCount: 1, VcpuOptions: []int{4, 6}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}},
		"h100_x1":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 1, VcpuOptions: []int{4, 8, 12, 16}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}},
		"h100_x2":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 2, VcpuOptions: []int{8, 12, 16, 20, 24}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"h100_x4":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 4, VcpuOptions: []int{60}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 2000}},
		"h100_x8":   {DisplayName: "NVIDIA H100", VramGB: 80, GpuCount: 8, VcpuOptions: []int{120}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 4000}},
		"l40_x1":    {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 1, VcpuOptions: []int{4, 8}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 500}},
		"l40_x2":    {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 2, VcpuOptions: []int{8, 12, 16}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"l40_x4":    {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 4, VcpuOptions: []int{40}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 2000}},
		"l40_x8":    {DisplayName: "NVIDIA L40", VramGB: 48, GpuCount: 8, VcpuOptions: []int{80}, RamPerVCPUGiB: 8, StorageGB: api.StorageRange{Min: 100, Max: 4000}},
	})
}

func TestGPUOptions(t *testing.T) {
	s := testSpecStore()

	got := s.GPUOptions()
	assert.Equal(t, []string{"a6000", "a100xl", "l40", "h100"}, got)
}

func TestGPUOptions_UnknownGPUAppended(t *testing.T) {
	s := NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1": {GpuCount: 1},
		"b200_x1":  {GpuCount: 1},
	})

	got := s.GPUOptions()
	// a6000 should come first (in display order), b200 appended after
	assert.Equal(t, "a6000", got[0])
	assert.Contains(t, got, "b200")
}

func TestGPUCounts(t *testing.T) {
	s := testSpecStore()

	tests := []struct {
		name     string
		gpuType  string
		expected []int
	}{
		{
			name:     "a6000 only has 1 GPU",
			gpuType:  "a6000",
			expected: []int{1},
		},
		{
			name:     "a100xl has 1, 2, 4, and 8 GPUs",
			gpuType:  "a100xl",
			expected: []int{1, 2, 4, 8},
		},
		{
			name:     "unknown GPU returns nil",
			gpuType:  "unknown",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.GPUCounts(tt.gpuType)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNeedsGPUCountPhase(t *testing.T) {
	s := testSpecStore()

	assert.False(t, s.NeedsGPUCountPhase("a6000"), "single-count GPU should not need count phase")
	assert.True(t, s.NeedsGPUCountPhase("a100xl"), "multi-count GPU should need count phase")
	assert.True(t, s.NeedsGPUCountPhase("h100"), "h100 has multiple counts")
}

func TestVCPUOptions(t *testing.T) {
	s := testSpecStore()

	assert.Equal(t, []int{4, 6}, s.VCPUOptions("a6000", 1))
	assert.Equal(t, []int{60}, s.VCPUOptions("a100xl", 4))
	assert.Nil(t, s.VCPUOptions("unknown", 1))
}

func TestIncludedVCPUs(t *testing.T) {
	s := testSpecStore()

	assert.Equal(t, 4, s.IncludedVCPUs("a6000", 1))
	assert.Equal(t, 8, s.IncludedVCPUs("a100xl", 2))
	assert.Equal(t, 4, s.IncludedVCPUs("unknown", 1), "fallback to 4")
}

func TestRamPerVCPU(t *testing.T) {
	s := testSpecStore()

	assert.Equal(t, 8, s.RamPerVCPU("a6000", 1))
	assert.Equal(t, 8, s.RamPerVCPU("a100xl", 1))
	assert.Equal(t, 8, s.RamPerVCPU("unknown", 1), "fallback")
}

func TestStorageRange(t *testing.T) {
	s := testSpecStore()

	tests := []struct {
		name        string
		gpuType     string
		numGPUs     int
		expectedMin int
		expectedMax int
	}{
		{"a6000", "a6000", 1, 100, 500},
		{"a100xl x1", "a100xl", 1, 100, 500},
		{"a100xl x2", "a100xl", 2, 100, 1000},
		{"unknown falls back to 100-1000", "unknown", 1, 100, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minGB, maxGB := s.StorageRange(tt.gpuType, tt.numGPUs)
			assert.Equal(t, tt.expectedMin, minGB)
			assert.Equal(t, tt.expectedMax, maxGB)
		})
	}
}

func TestNormalizeGPUType(t *testing.T) {
	s := testSpecStore()

	tests := []struct {
		name    string
		input   string
		wantGPU string
		wantOK  bool
	}{
		{"exact match", "a6000", "a6000", true},
		{"a100 alias", "a100", "a100xl", true},
		{"uppercase normalized", "A100", "a100xl", true},
		{"unknown GPU", "v100", "v100", false},
		{"h100", "h100", "h100", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpu, ok := s.NormalizeGPUType(tt.input)
			assert.Equal(t, tt.wantGPU, gpu)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestLookup(t *testing.T) {
	s := testSpecStore()

	spec := s.Lookup("a6000", 1)
	assert.NotNil(t, spec)
	assert.Equal(t, "RTX A6000", spec.DisplayName)
	assert.Equal(t, 48, spec.VramGB)

	assert.Nil(t, s.Lookup("unknown", 1))
	assert.Nil(t, s.Lookup("a6000", 2))
}
