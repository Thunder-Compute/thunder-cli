package utils

import (
	"fmt"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
)

// SpecStore wraps fetched GPU specs and provides helper methods
// that replace the old hardcoded prototyping/production config.
type SpecStore struct {
	specs        map[string]api.GpuSpecConfig
	availability map[string]string
}

// NewSpecStore creates a SpecStore from API-fetched specs.
func NewSpecStore(specs map[string]api.GpuSpecConfig) *SpecStore {
	return &SpecStore{specs: specs}
}

// NewSpecStoreWithAvailability creates a SpecStore with optional per-spec availability.
func NewSpecStoreWithAvailability(specs map[string]api.GpuSpecConfig, availability map[string]string) *SpecStore {
	return &SpecStore{specs: specs, availability: availability}
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

// IsSpecAvailable reports whether a concrete spec is available. Availability
// fails open when the API did not provide availability data.
func (s *SpecStore) IsSpecAvailable(gpuType string, gpuCount int, mode string) bool {
	if s == nil || len(s.availability) == 0 {
		return true
	}
	return s.availability[configKey(gpuType, gpuCount, mode)] == "available"
}

// IsGPUTypeAvailableForMode reports whether any count for this GPU type is available.
func (s *SpecStore) IsGPUTypeAvailableForMode(gpuType string, mode string) bool {
	for _, count := range s.GPUCountsForMode(gpuType, mode) {
		if s.IsSpecAvailable(gpuType, count, mode) {
			return true
		}
	}
	return false
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

// EphemeralStorageRange returns the min/max ephemeral storage for a configuration.
func (s *SpecStore) EphemeralStorageRange(gpuType string, numGPUs int, mode string) (int, int) {
	spec := s.Lookup(gpuType, numGPUs, mode)
	if spec == nil {
		return 0, 2000
	}
	return spec.EphemeralStorageGB.Min, spec.EphemeralStorageGB.Max
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
