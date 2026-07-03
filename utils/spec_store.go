package utils

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/Thunder-Compute/thunder-cli/api"
)

// SpecStore wraps fetched GPU specs and provides helper methods.
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

// FetchSpecStore loads GPU specs and availability from the API concurrently and
// merges them into a SpecStore. Specs are required; availability is best-effort
// — a failure leaves configurations unmarked rather than aborting.
func FetchSpecStore(client *api.Client) (*SpecStore, error) {
	var (
		specsMap     map[string]api.GpuSpecConfig
		specsErr     error
		availability map[string]string
		wg           sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		specsMap, specsErr = client.GetSpecs()
	}()
	go func() {
		defer wg.Done()
		if avail, err := client.GetAvailability(); err == nil && avail != nil {
			availability = avail.Specs
		}
	}()
	wg.Wait()

	if specsErr != nil {
		return nil, fmt.Errorf("failed to fetch GPU specs: %w", specsErr)
	}
	return NewSpecStoreWithAvailability(specsMap, availability), nil
}

func publicConfigKey(gpuType string, gpuCount int) string {
	return fmt.Sprintf("%s_x%d", gpuType, gpuCount)
}

// Lookup returns the spec for a given GPU type and count.
func (s *SpecStore) Lookup(gpuType string, gpuCount int) *api.GpuSpecConfig {
	spec, ok := s.specs[publicConfigKey(gpuType, gpuCount)]
	if ok {
		return &spec
	}
	return nil
}

// IsSpecAvailable reports whether a concrete spec is available. Availability
// fails open when the API did not provide availability data.
func (s *SpecStore) IsSpecAvailable(gpuType string, gpuCount int) bool {
	if s == nil || len(s.availability) == 0 {
		return true
	}
	status, ok := s.availability[publicConfigKey(gpuType, gpuCount)]
	if !ok {
		return true
	}
	return status == "available"
}

// gpuDisplayOrder defines the canonical display ordering for GPU types
// (ascending by cost/performance).
var gpuDisplayOrder = []string{"a6000", "a100xl", "l40", "l40s", "h100"}

// GPUOptions returns the GPU type identifiers available in any size,
// ordered by gpuDisplayOrder. Out-of-stock types are preserved in the list but
// pushed to the bottom.
func (s *SpecStore) GPUOptions() []string {
	seen := map[string]bool{}
	for key := range s.specs {
		gpuType := strings.Split(key, "_x")[0]
		seen[gpuType] = true
	}
	var ordered []string
	for _, gpu := range gpuDisplayOrder {
		if seen[gpu] {
			ordered = append(ordered, gpu)
		}
	}
	for gpuType := range seen {
		if !slices.Contains(gpuDisplayOrder, gpuType) {
			ordered = append(ordered, gpuType)
		}
	}
	var available, unavailable []string
	for _, gpu := range ordered {
		if s.IsGPUTypeAvailable(gpu) {
			available = append(available, gpu)
		} else {
			unavailable = append(unavailable, gpu)
		}
	}
	return append(available, unavailable...)
}

// GPUCounts returns all valid GPU counts for a given GPU type, sorted.
func (s *SpecStore) GPUCounts(gpuType string) []int {
	var counts []int
	for gpuCount := 1; gpuCount <= 8; gpuCount++ {
		if s.Lookup(gpuType, gpuCount) != nil {
			counts = append(counts, gpuCount)
		}
	}
	return counts
}

// IsGPUTypeAvailable reports whether any count for this GPU type is available.
func (s *SpecStore) IsGPUTypeAvailable(gpuType string) bool {
	for _, count := range s.GPUCounts(gpuType) {
		if s.IsSpecAvailable(gpuType, count) {
			return true
		}
	}
	return false
}

// VCPUOptions returns the allowed vCPU counts for a configuration.
func (s *SpecStore) VCPUOptions(gpuType string, numGPUs int) []int {
	spec := s.Lookup(gpuType, numGPUs)
	if spec == nil {
		return nil
	}
	return spec.VcpuOptions
}

// NeedsGPUCountPhase reports whether the GPU type supports multiple GPU counts.
func (s *SpecStore) NeedsGPUCountPhase(gpuType string) bool {
	return len(s.GPUCounts(gpuType)) > 1
}

// IncludedVCPUs returns the minimum (included) vCPU count for a configuration.
func (s *SpecStore) IncludedVCPUs(gpuType string, numGPUs int) int {
	opts := s.VCPUOptions(gpuType, numGPUs)
	if len(opts) == 0 {
		return 4 // safe default
	}
	return opts[0]
}

// RamPerVCPU returns the RAM per vCPU in GiB for a configuration.
func (s *SpecStore) RamPerVCPU(gpuType string, numGPUs int) int {
	spec := s.Lookup(gpuType, numGPUs)
	if spec == nil {
		return 8
	}
	return spec.RamPerVCPUGiB
}

// RamForVCPU returns total system RAM in GiB for a given vCPU count. When the
// config sets ramCapGiB, RAM is affine (capped at the max vCPU option and reduced
// by RamPerVCPUGiB per vCPU below it); otherwise it is vcpus * RamPerVCPUGiB.
// Mirrors thundertypes.GpuSpecConfig.MemoryGiB.
func (s *SpecStore) RamForVCPU(gpuType string, numGPUs, vcpus int) int {
	spec := s.Lookup(gpuType, numGPUs)
	if spec == nil {
		return vcpus * 8
	}
	if spec.RamCapGiB > 0 && len(spec.VcpuOptions) > 0 {
		maxVcpu := spec.VcpuOptions[0]
		for _, v := range spec.VcpuOptions {
			if v > maxVcpu {
				maxVcpu = v
			}
		}
		return spec.RamCapGiB - (maxVcpu-vcpus)*spec.RamPerVCPUGiB
	}
	return vcpus * spec.RamPerVCPUGiB
}

// StorageRange returns the min/max storage for a configuration.
func (s *SpecStore) StorageRange(gpuType string, numGPUs int) (int, int) {
	spec := s.Lookup(gpuType, numGPUs)
	if spec == nil {
		return 100, 1000
	}
	return spec.StorageGB.Min, spec.StorageGB.Max
}

// NormalizeGPUType maps user-friendly GPU names to canonical names,
// validated against available specs.
// Returns the canonical name and whether it was found.
func (s *SpecStore) NormalizeGPUType(input string) (string, bool) {
	input = strings.ToLower(input)

	// Common aliases
	aliases := map[string]string{
		"a100": "a100xl",
	}
	if canonical, ok := aliases[input]; ok {
		input = canonical
	}

	// Verify this GPU type exists.
	for _, gpu := range s.GPUOptions() {
		if gpu == input {
			return input, true
		}
	}
	return input, false
}
