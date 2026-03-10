package utils

import "fmt"

// PricingData holds fetched pricing rates from the API.
type PricingData struct {
	Rates map[string]float64
}

// gpuPricingKey returns the pricing map key for the given GPU configuration.
// Format: "{gpu}_x{count}_{mode}", e.g. "h100_x1_prototyping", "a100xl_x4_production".
func gpuPricingKey(mode, gpuType string, numGPUs int) string {
	return fmt.Sprintf("%s_x%d_%s", gpuType, numGPUs, mode)
}

// CalculateHourlyPrice computes the estimated hourly cost based on the configuration.
// includedVCPUs is the minimum (included) vCPU count from specs (vcpuOptions[0]).
func CalculateHourlyPrice(p *PricingData, mode, gpuType string, numGPUs, vcpus, diskSizeGB, includedVCPUs int) float64 {
	if p == nil || p.Rates == nil {
		return 0
	}

	gpuCost := p.Rates[gpuPricingKey(mode, gpuType, numGPUs)]

	var vcpuCost float64
	if mode == "prototyping" {
		included := includedVCPUs
		if included == 0 {
			included = 4
		}
		extra := max(0, vcpus-included)
		if extra > 0 {
			rate := p.Rates["additional_vcpus"]
			// Cores up to 32 total at normal rate, beyond 32 at 1.5x rate
			coresAtNormal := max(0, min(extra, 32-included))
			coresBeyond := extra - coresAtNormal
			vcpuCost = float64(coresAtNormal)*rate + float64(coresBeyond)*rate*1.5
		}
	}

	var diskCost float64
	if diskSizeGB > 100 {
		diskCost = float64(diskSizeGB-100) * p.Rates["disk_gb"]
	}

	return gpuCost + vcpuCost + diskCost
}

// FormatPrice returns a display string like "$1.38/hr".
func FormatPrice(hourlyPrice float64) string {
	return fmt.Sprintf("$%.2f/hr", hourlyPrice)
}
