package utils

import "fmt"

// PricingData holds fetched pricing rates from the API.
type PricingData struct {
	Rates map[string]float64
}

func publicGPUPricingKey(gpuType string, numGPUs int) string {
	return fmt.Sprintf("%s_x%d", gpuType, numGPUs)
}

// CalculateHourlyPrice computes the estimated hourly cost based on the configuration.
// includedVCPUs is the minimum (included) vCPU count from specs (vcpuOptions[0]).
func CalculateHourlyPrice(p *PricingData, gpuType string, numGPUs, vcpus, diskSizeGB, includedVCPUs int) float64 {
	if p == nil || p.Rates == nil {
		return 0
	}

	gpuCost := p.Rates[publicGPUPricingKey(gpuType, numGPUs)]

	included := includedVCPUs
	if included == 0 {
		included = 4
	}
	var vcpuCost float64
	extra := max(0, vcpus-included)
	if extra > 0 {
		rate := p.Rates["additional_vcpus"]
		vcpuCost = float64(extra) * rate
	}

	// First 100GB of persistent disk per GPU is included free.
	var diskCost float64
	includedDiskGB := numGPUs * 100
	if diskSizeGB > includedDiskGB {
		diskCost = float64(diskSizeGB-includedDiskGB) * p.Rates["disk_gb"]
	}

	return gpuCost + vcpuCost + diskCost
}

// FormatPrice returns a display string like "$1.38/hr".
func FormatPrice(hourlyPrice float64) string {
	return fmt.Sprintf("$%.2f/hr", hourlyPrice)
}
