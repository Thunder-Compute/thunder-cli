package utils

import "testing"

func TestFormatGPUTypeUsesConciseLabels(t *testing.T) {
	tests := map[string]string{
		"a6000":  "A6000",
		"a100xl": "A100 80GB",
		"l40":    "L40",
		"h100":   "H100",
	}

	for gpuType, want := range tests {
		t.Run(gpuType, func(t *testing.T) {
			if got := FormatGPUType(gpuType); got != want {
				t.Fatalf("FormatGPUType(%q) = %q, want %q", gpuType, got, want)
			}
		})
	}
}
