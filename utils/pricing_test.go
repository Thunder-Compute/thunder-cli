package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPricingData() *PricingData {
	return &PricingData{
		Rates: map[string]float64{
			"a6000_x1_prototyping":  0.50,
			"a100xl_x1_prototyping": 1.10,
			"a100xl_x2_prototyping": 2.20,
			"h100_x1_prototyping":   2.49,
			"h100_x2_prototyping":   4.98,
			"a100xl_x1_production":  1.64,
			"h100_x1_production":    3.49,
			"additional_vcpus":      0.03,
			"disk_gb":               0.0001,
			"ephemeral_disk_gb":     0.0002,
		},
	}
}

func TestCalculateHourlyPrice(t *testing.T) {
	p := testPricingData()

	tests := []struct {
		name            string
		pricing         *PricingData
		mode            string
		gpuType         string
		numGPUs         int
		vcpus           int
		diskSizeGB      int
		ephemeralDiskGB int
		includedVCPU    int
		expected        float64
	}{
		{
			name:         "nil pricing returns zero",
			pricing:      nil,
			mode:         "prototyping",
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
			mode:         "prototyping",
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
			mode:         "prototyping",
			gpuType:      "a6000",
			numGPUs:      1,
			vcpus:        4,
			diskSizeGB:   100,
			includedVCPU: 4,
			expected:     0.50,
		},
		{
			name:         "extra vCPUs in prototyping mode",
			pricing:      p,
			mode:         "prototyping",
			gpuType:      "a6000",
			numGPUs:      1,
			vcpus:        8,
			diskSizeGB:   100,
			includedVCPU: 4,
			// 4 extra vCPUs * 0.03 = 0.12
			expected: 0.50 + 0.12,
		},
		{
			name:         "production mode with included vCPUs has no surcharge",
			pricing:      p,
			mode:         "production",
			gpuType:      "a100xl",
			numGPUs:      1,
			vcpus:        18,
			diskSizeGB:   100,
			includedVCPU: 18,
			expected:     1.64,
		},
		{
			name:         "disk surcharge above 100GB",
			pricing:      p,
			mode:         "prototyping",
			gpuType:      "a6000",
			numGPUs:      1,
			vcpus:        4,
			diskSizeGB:   300,
			includedVCPU: 4,
			// 200 extra GB * 0.0001 = 0.02
			expected: 0.50 + 0.02,
		},
		{
			name:         "no disk surcharge at exactly 100GB",
			pricing:      p,
			mode:         "prototyping",
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
			mode:         "prototyping",
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
			mode:         "prototyping",
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
			mode:         "prototyping",
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
			mode:         "prototyping",
			gpuType:      "unknown",
			numGPUs:      1,
			vcpus:        4,
			diskSizeGB:   100,
			includedVCPU: 4,
			expected:     0,
		},
		{
			name:            "ephemeral disk cost",
			pricing:         p,
			mode:            "prototyping",
			gpuType:         "a6000",
			numGPUs:         1,
			vcpus:           4,
			diskSizeGB:      100,
			ephemeralDiskGB: 200,
			includedVCPU:    4,
			// 200 * 0.0002 = 0.04
			expected: 0.50 + 0.04,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateHourlyPrice(tt.pricing, tt.mode, tt.gpuType, tt.numGPUs, tt.vcpus, tt.diskSizeGB, tt.ephemeralDiskGB, tt.includedVCPU)
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
