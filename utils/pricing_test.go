package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPricingData() *PricingData {
	return &PricingData{
		Rates: map[string]float64{
			"a6000_x1":         0.50,
			"a100xl_x1":        1.10,
			"a100xl_x2":        2.20,
			"h100_x1":          2.49,
			"h100_x2":          4.98,
			"additional_vcpus": 0.03,
			"disk_gb":          0.0001,
		},
	}
}

func TestCalculateHourlyPrice(t *testing.T) {
	p := testPricingData()

	tests := []struct {
		name         string
		pricing      *PricingData
		gpuType      string
		numGPUs      int
		vcpus        int
		diskSizeGB   int
		includedVCPU int
		expected     float64
	}{
		{
			name:         "nil pricing returns zero",
			pricing:      nil,
			gpuType:      "a6000",
			numGPUs:      1,
			vcpus:        4,
			diskSizeGB:   100,
			includedVCPU: 4,
			expected:     0,
		},
		{
			name:         "nil rates returns zero",
			pricing:      &PricingData{Rates: nil},
			gpuType:      "a6000",
			numGPUs:      1,
			vcpus:        4,
			diskSizeGB:   100,
			includedVCPU: 4,
			expected:     0,
		},
		{
			name:         "base GPU cost only, no extras",
			pricing:      p,
			gpuType:      "a6000",
			numGPUs:      1,
			vcpus:        4,
			diskSizeGB:   100,
			includedVCPU: 4,
			expected:     0.50,
		},
		{
			name:         "extra vCPUs",
			pricing:      p,
			gpuType:      "a6000",
			numGPUs:      1,
			vcpus:        8,
			diskSizeGB:   100,
			includedVCPU: 4,
			// 4 extra vCPUs * 0.03 = 0.12
			expected: 0.50 + 0.12,
		},
		{
			name:         "included vCPUs have no surcharge",
			pricing:      p,
			gpuType:      "a100xl",
			numGPUs:      1,
			vcpus:        18,
			diskSizeGB:   100,
			includedVCPU: 18,
			expected:     1.10,
		},
		{
			name:         "disk surcharge above 100GB",
			pricing:      p,
			gpuType:      "a6000",
			numGPUs:      1,
			vcpus:        4,
			diskSizeGB:   300,
			includedVCPU: 4,
			// 200 extra GB * 0.0001 = 0.02
			expected: 0.50 + 0.02,
		},
		{
			name:         "disk allowance is 100GB per GPU: 2 GPUs, 200GB free",
			pricing:      p,
			gpuType:      "a100xl",
			numGPUs:      2,
			vcpus:        8,
			diskSizeGB:   200,
			includedVCPU: 8,
			// 200GB <= 2*100GB free → no disk surcharge; a100xl_x2 base only
			expected: 2.20,
		},
		{
			name:         "disk surcharge above per-GPU allowance: 2 GPUs, 300GB",
			pricing:      p,
			gpuType:      "a100xl",
			numGPUs:      2,
			vcpus:        8,
			diskSizeGB:   300,
			includedVCPU: 8,
			// 300 - 2*100 = 100 extra GB * 0.0001 = 0.01
			expected: 2.20 + 0.01,
		},
		{
			name:         "no disk surcharge at exactly 100GB",
			pricing:      p,
			gpuType:      "h100",
			numGPUs:      1,
			vcpus:        4,
			diskSizeGB:   100,
			includedVCPU: 4,
			expected:     2.49,
		},
		{
			name:         "vCPU pricing: beyond 32 total at flat rate",
			pricing:      p,
			gpuType:      "a100xl",
			numGPUs:      2,
			vcpus:        40,
			diskSizeGB:   100,
			includedVCPU: 8,
			// extra = 40-8 = 32
			// vcpuCost = 32 * 0.03 = 0.96
			expected: 2.20 + 0.96,
		},
		{
			name:         "all extras combined",
			pricing:      p,
			gpuType:      "h100",
			numGPUs:      1,
			vcpus:        12,
			diskSizeGB:   500,
			includedVCPU: 4,
			// extra vCPUs = 8 * 0.03 = 0.24
			// extra disk = 400 * 0.0001 = 0.04
			expected: 2.49 + 0.24 + 0.04,
		},
		{
			name:         "includedVCPUs defaults to 4 when zero",
			pricing:      p,
			gpuType:      "a6000",
			numGPUs:      1,
			vcpus:        8,
			diskSizeGB:   100,
			includedVCPU: 0,
			// included defaults to 4, extra = 4 * 0.03 = 0.12
			expected: 0.50 + 0.12,
		},
		{
			name:         "unknown GPU type returns zero base cost",
			pricing:      p,
			gpuType:      "unknown",
			numGPUs:      1,
			vcpus:        4,
			diskSizeGB:   100,
			includedVCPU: 4,
			expected:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateHourlyPrice(tt.pricing, tt.gpuType, tt.numGPUs, tt.vcpus, tt.diskSizeGB, tt.includedVCPU)
			assert.InDelta(t, tt.expected, got, 0.001)
		})
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		price    float64
		expected string
	}{
		{0, "$0.00/hr"},
		{1.5, "$1.50/hr"},
		{0.123, "$0.12/hr"},
		{10.999, "$11.00/hr"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatPrice(tt.price))
		})
	}
}
