package utils

import "strings"

func Capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// FormatGPUType converts internal GPU type codes to user-facing display names
func FormatGPUType(gpuType string) string {
	switch strings.ToLower(gpuType) {
	case "a6000":
		return "A6000"
	case "a100xl":
		return "A100 80GB"
	case "h100":
		return "H100"
	default:
		return gpuType
	}
}
