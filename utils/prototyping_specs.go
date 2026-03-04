package utils

// PrototypingSpecSpace defines the valid vCPU options for each GPU type and
// count in prototyping mode. Keys use the CLI representation (lowercase).
// The first element of each slice is the minimum (included) vCPU count.
var PrototypingSpecSpace = map[string]map[int][]int{
	"a6000":  {1: {4, 8}},
	"a100xl": {1: {4, 8, 12}, 2: {8, 12, 16, 20, 24}},
	"h100":   {1: {4, 8, 12, 16}, 2: {8, 12, 16, 20, 24}},
}

// PrototypingGPUOptions returns the GPU types available in prototyping mode.
func PrototypingGPUOptions() []string {
	return []string{"a6000", "a100xl", "h100"}
}

// PrototypingVCPUOptions returns the allowed vCPU counts for a prototyping
// configuration. Returns nil if the GPU type / count combination is invalid.
func PrototypingVCPUOptions(gpuType string, numGPUs int) []int {
	counts, ok := PrototypingSpecSpace[gpuType]
	if !ok {
		return nil
	}
	vcpus, ok := counts[numGPUs]
	if !ok {
		return nil
	}
	return vcpus
}

// IncludedVCPUs returns the minimum (included) vCPU count for a prototyping
// configuration. Returns 4 as a safe default if the combination is unknown.
func IncludedVCPUs(gpuType string, numGPUs int) int {
	vcpus := PrototypingVCPUOptions(gpuType, numGPUs)
	if len(vcpus) == 0 {
		return 4
	}
	return vcpus[0]
}

// NeedsGPUCountPhase reports whether the GPU type supports multi-GPU in
// prototyping mode and should show a GPU count selection step.
func NeedsGPUCountPhase(gpuType string) bool {
	counts, ok := PrototypingSpecSpace[gpuType]
	if !ok {
		return false
	}
	return len(counts) > 1
}

// PrototypingGPUCounts returns the valid GPU counts for a prototyping GPU type.
func PrototypingGPUCounts(gpuType string) []int {
	counts, ok := PrototypingSpecSpace[gpuType]
	if !ok {
		return []int{1}
	}
	result := make([]int, 0, len(counts))
	for k := range counts {
		result = append(result, k)
	}
	// Sort ascending
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j] < result[i] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}
