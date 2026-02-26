package tui

import "fmt"

// PricingData holds fetched pricing rates from the API.
type PricingData struct {
	Rates map[string]float64
}

// includedVCPUs returns the minimum (included) vCPU count for a given GPU type and count in prototyping mode.
func includedVCPUs(gpuType string, numGPUs int) int {
	switch gpuType {
	case "a6000":
		return 4
	case "a100xl":
		return 4
	case "h100":
		if numGPUs >= 2 {
			return 8
		}
		return 4
	default:
		return 4
	}
}

// CalculateHourlyPrice computes the estimated hourly cost based on the configuration.
func CalculateHourlyPrice(p *PricingData, mode, gpuType string, numGPUs, vcpus, diskSizeGB int) float64 {
	if p == nil || p.Rates == nil {
		return 0
	}

	var gpuCost float64
	if mode == "production" {
		gpuCost = p.Rates[gpuType+"_native"] * float64(numGPUs)
	} else {
		gpuCost = p.Rates[gpuType] * float64(numGPUs)
	}

	var vcpuCost float64
	if mode == "prototyping" {
		included := includedVCPUs(gpuType, numGPUs)
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
