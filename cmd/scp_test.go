package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePathWithOS(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		goos     string
		expected PathInfo
	}{
		{
			name: "numeric instance ID with remote path on windows",
			path: "0:/home/ubuntu/",
			goos: "windows",
			expected: PathInfo{
				Original:   "0:/home/ubuntu/",
				InstanceID: "0",
				Path:       "/home/ubuntu/",
				IsRemote:   true,
			},
		},
		{
			name: "named instance ID with remote path on windows",
			path: "MAVERICK:/home/ubuntu/file.txt",
			goos: "windows",
			expected: PathInfo{
				Original:   "MAVERICK:/home/ubuntu/file.txt",
				InstanceID: "MAVERICK",
				Path:       "/home/ubuntu/file.txt",
				IsRemote:   true,
			},
		},
		{
			name: "windows drive letter C: should be local",
			path: `C:\Users\test\file.txt`,
			goos: "windows",
			expected: PathInfo{
				Original: `C:\Users\test\file.txt`,
				Path:     `C:\Users\test\file.txt`,
			},
		},
		{
			name: "windows drive letter D: should be local",
			path: `D:\data\file.txt`,
			goos: "windows",
			expected: PathInfo{
				Original: `D:\data\file.txt`,
				Path:     `D:\data\file.txt`,
			},
		},
		{
			name: "local relative path on windows",
			path: `.\cloud-comfyui.sh`,
			goos: "windows",
			expected: PathInfo{
				Original: `.\cloud-comfyui.sh`,
				Path:     `.\cloud-comfyui.sh`,
			},
		},
		{
			name: "numeric instance ID on linux",
			path: "0:/home/ubuntu/",
			goos: "linux",
			expected: PathInfo{
				Original:   "0:/home/ubuntu/",
				InstanceID: "0",
				Path:       "/home/ubuntu/",
				IsRemote:   true,
			},
		},
		{
			name: "local path on linux",
			path: "./file.txt",
			goos: "linux",
			expected: PathInfo{
				Original: "./file.txt",
				Path:     "./file.txt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePathWithOS(tt.path, tt.goos)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineTransferDirection_WindowsNumericID(t *testing.T) {
	// Simulates the exact user scenario: tnr scp ./cloud-comfyui.sh 0:/home/ubuntu/
	// on Windows. The destination should be recognized as remote.
	source := parsePathWithOS(`.\cloud-comfyui.sh`, "windows")
	dest := parsePathWithOS("0:/home/ubuntu/", "windows")

	direction, instanceID, err := determineTransferDirection([]PathInfo{source}, dest)
	assert.NoError(t, err, "should not error - destination is a valid remote path")
	assert.Equal(t, "upload", direction)
	assert.Equal(t, "0", instanceID)
}
