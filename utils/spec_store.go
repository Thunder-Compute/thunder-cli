package utils

import (
	"fmt"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
)

// SpecStore wraps fetched GPU specs and provides helper methods
// that replace the old hardcoded prototyping/production config.
type SpecStore struct {
	specs map[string]api.GpuSpecConfig
}

// NewSpecStore creates a SpecStore from API-fetched specs.
func NewSpecStore(specs map[string]api.GpuSpecConfig) *SpecStore {
	return &SpecStore{specs: specs}
}

func configKey(gpuType string, gpuCount int, mode string) string {
	return fmt.Sprintf("%s_x%d_%s", gpuType, gpuCount, mode)
}

// Lookup returns the spec for a given GPU type, count, and mode.
func (s *SpecStore) Lookup(gpuType string, gpuCount int, mode string) *api.GpuSpecConfig {
	key := configKey(gpuType, gpuCount, mode)
	spec, ok := s.specs[key]
	if !ok {
		return nil
	}
	return &spec
}

// gpuDisplayOrder defines the canonical display ordering for GPU types
// (ascending by cost/performance).
var gpuDisplayOrder = []string{"a6000", "a100xl", "h100"}

// GPUOptionsForMode returns the GPU type identifiers available for a mode,
// ordered by gpuDisplayOrder (a6000, a100xl, h100).
func (s *SpecStore) GPUOptionsForMode(mode string) []string {
	seen := map[string]bool{}
	for key, spec := range s.specs {
		if spec.Mode == mode {
			gpuType := key[:len(key)-len(fmt.Sprintf("_x%d_%s", spec.GpuCount, spec.Mode))]
			seen[gpuType] = true
		}
	}
	var types []string
	for _, gpu := range gpuDisplayOrder {
		if seen[gpu] {
			types = append(types, gpu)
		}
	}
	// Append any GPU types not in the predefined order
	for gpuType := range seen {
		found := false
		for _, g := range gpuDisplayOrder {
			if g == gpuType {
				found = true
				break
			}
		}
		if !found {
			types = append(types, gpuType)
		}
	}
	return types
}

// GPUCountsForMode returns all valid GPU counts for a given GPU type and mode, sorted.
func (s *SpecStore) GPUCountsForMode(gpuType string, mode string) []int {
	var counts []int
	for gpuCount := 1; gpuCount <= 8; gpuCount++ {
		if _, ok := s.specs[configKey(gpuType, gpuCount, mode)]; ok {
			counts = append(counts, gpuCount)
		}
	}
	return counts
}

// VCPUOptions returns the allowed vCPU counts for a configuration.
func (s *SpecStore) VCPUOptions(gpuType string, numGPUs int, mode string) []int {
	spec := s.Lookup(gpuType, numGPUs, mode)
	if spec == nil {
		return nil
	}
	return spec.VcpuOptions
}

// NeedsGPUCountPhase reports whether the GPU type supports multiple GPU counts.
func (s *SpecStore) NeedsGPUCountPhase(gpuType string, mode string) bool {
	return len(s.GPUCountsForMode(gpuType, mode)) > 1
}

// IncludedVCPUs returns the minimum (included) vCPU count for a configuration.
func (s *SpecStore) IncludedVCPUs(gpuType string, numGPUs int, mode string) int {
	opts := s.VCPUOptions(gpuType, numGPUs, mode)
	if len(opts) == 0 {
		return 4 // safe default
	}
	return opts[0]
}

// RamPerVCPU returns the RAM per vCPU in GiB for a configuration.
func (s *SpecStore) RamPerVCPU(gpuType string, numGPUs int, mode string) int {
	spec := s.Lookup(gpuType, numGPUs, mode)
	if spec == nil {
		if mode == "production" {
			return 5
		}
		return 8
	}
	return spec.RamPerVCPUGiB
}

// StorageRange returns the min/max storage for a configuration.
func (s *SpecStore) StorageRange(gpuType string, numGPUs int, mode string) (int, int) {
	spec := s.Lookup(gpuType, numGPUs, mode)
	if spec == nil {
		return 100, 1000
	}
	return spec.StorageGB.Min, spec.StorageGB.Max
}

// StorageOptions returns valid disk size options in 100GB increments for a configuration.
func (s *SpecStore) StorageOptions(gpuType string, numGPUs int, mode string) []int {
	minGB, maxGB := s.StorageRange(gpuType, numGPUs, mode)
	var options []int
	for i := minGB; i <= maxGB; i += 100 {
		options = append(options, i)
	}
	return options
}

// NormalizeGPUType maps user-friendly GPU names to canonical names,
// validated against available specs for the given mode.
// Returns the canonical name and whether it was found.
func (s *SpecStore) NormalizeGPUType(input string, mode string) (string, bool) {
	input = strings.ToLower(input)

	// Common aliases
	aliases := map[string]string{
		"a100": "a100xl",
	}
	if canonical, ok := aliases[input]; ok {
		input = canonical
	}

	// Verify this GPU type exists for the given mode
	for _, gpu := range s.GPUOptionsForMode(mode) {
		if gpu == input {
			return input, true
		}
	}
	return input, false
}
