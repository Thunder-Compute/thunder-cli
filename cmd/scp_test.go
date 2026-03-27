package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestParsePathWithOS_AdditionalCases(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		goos       string
		wantRemote bool
		wantID     string
		wantPath   string
	}{
		{
			name:       "dots in identifier prevent remote detection",
			path:       "my.host:/path",
			goos:       "linux",
			wantRemote: false,
			wantPath:   "my.host:/path",
		},
		{
			name:       "slash in identifier prevents remote detection",
			path:       "path/to:/something",
			goos:       "linux",
			wantRemote: false,
			wantPath:   "path/to:/something",
		},
		{
			name:       "tilde path is local",
			path:       "~/documents/file.txt",
			goos:       "linux",
			wantRemote: false,
			wantPath:   "~/documents/file.txt",
		},
		{
			name:       "remote with empty path",
			path:       "inst123:",
			goos:       "linux",
			wantRemote: true,
			wantID:     "inst123",
			wantPath:   "",
		},
		{
			name:       "instance ID at max length (20 chars)",
			path:       "12345678901234567890:/data",
			goos:       "linux",
			wantRemote: true,
			wantID:     "12345678901234567890",
			wantPath:   "/data",
		},
		{
			name:       "instance ID too long (21 chars) not remote",
			path:       "123456789012345678901:/data",
			goos:       "linux",
			wantRemote: false,
			wantPath:   "123456789012345678901:/data",
		},
		{
			name:       "lowercase windows drive on windows",
			path:       "d:\\data\\file.txt",
			goos:       "windows",
			wantRemote: false,
			wantPath:   "d:\\data\\file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePathWithOS(tt.path, tt.goos)
			assert.Equal(t, tt.path, got.Original)
			assert.Equal(t, tt.wantRemote, got.IsRemote, "IsRemote")
			assert.Equal(t, tt.wantID, got.InstanceID, "InstanceID")
			assert.Equal(t, tt.wantPath, got.Path, "Path")
		})
	}
}

func TestIsValidInstanceID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"inst123", true},
		{"abc", true},
		{"a", true},
		{"0", true},
		{"inst-123", true},
		{"inst_123", true},
		{"12345678901234567890", true},  // exactly 20 chars
		{"123456789012345678901", false}, // 21 chars - too long
		{"", false},
		{"has/slash", false},
		{"has\\backslash", false},
		{"has.dot", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidInstanceID(tt.input))
		})
	}
}

func TestDetermineTransferDirection(t *testing.T) {
	local := func(path string) PathInfo {
		return PathInfo{Original: path, Path: path, IsRemote: false}
	}
	remote := func(id, path string) PathInfo {
		return PathInfo{Original: id + ":" + path, InstanceID: id, Path: path, IsRemote: true}
	}

	tests := []struct {
		name          string
		sources       []PathInfo
		dest          PathInfo
		wantDir       string
		wantID        string
		expectError   bool
		errorContains string
	}{
		{
			name:    "upload: single local to remote",
			sources: []PathInfo{local("file.txt")},
			dest:    remote("inst1", "/home/user/"),
			wantDir: "upload",
			wantID:  "inst1",
		},
		{
			name:    "upload: multiple local to remote",
			sources: []PathInfo{local("a.txt"), local("b.txt")},
			dest:    remote("inst1", "/home/user/"),
			wantDir: "upload",
			wantID:  "inst1",
		},
		{
			name:    "download: single remote to local",
			sources: []PathInfo{remote("inst1", "/data/file.txt")},
			dest:    local("./"),
			wantDir: "download",
			wantID:  "inst1",
		},
		{
			name:    "download: multiple remote same instance",
			sources: []PathInfo{remote("inst1", "/a.txt"), remote("inst1", "/b.txt")},
			dest:    local("./"),
			wantDir: "download",
			wantID:  "inst1",
		},
		{
			name:          "error: remote to remote",
			sources:       []PathInfo{remote("inst1", "/a.txt")},
			dest:          remote("inst2", "/b/"),
			expectError:   true,
			errorContains: "cannot transfer from remote to remote",
		},
		{
			name:          "error: multiple different instances",
			sources:       []PathInfo{remote("inst1", "/a.txt"), remote("inst2", "/b.txt")},
			dest:          local("./"),
			expectError:   true,
			errorContains: "cannot transfer between multiple instances",
		},
		{
			name:          "error: no remote path at all",
			sources:       []PathInfo{local("a.txt")},
			dest:          local("./"),
			expectError:   true,
			errorContains: "no remote path specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, id, err := determineTransferDirection(tt.sources, tt.dest)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantDir, dir)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}

func TestDetermineTransferDirection_WindowsNumericID(t *testing.T) {
	source := parsePathWithOS(`.\cloud-comfyui.sh`, "windows")
	dest := parsePathWithOS("0:/home/ubuntu/", "windows")

	direction, instanceID, err := determineTransferDirection([]PathInfo{source}, dest)
	assert.NoError(t, err)
	assert.Equal(t, "upload", direction)
	assert.Equal(t, "0", instanceID)
}

func TestDetermineTransferDirection(t *testing.T) {
	local := func(path string) PathInfo { return PathInfo{Original: path, Path: path} }
	remote := func(id, path string) PathInfo {
		return PathInfo{Original: id + ":" + path, InstanceID: id, Path: path, IsRemote: true}
	}

	tests := []struct {
		name          string
		sources       []PathInfo
		dest          PathInfo
		wantDir       string
		wantInstance  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "upload: local source, remote dest",
			sources:      []PathInfo{local("./file.txt")},
			dest:         remote("inst1", "/home/ubuntu/"),
			wantDir:      "upload",
			wantInstance: "inst1",
		},
		{
			name:         "download: remote source, local dest",
			sources:      []PathInfo{remote("inst1", "/data/file.csv")},
			dest:         local("./downloads/"),
			wantDir:      "download",
			wantInstance: "inst1",
		},
		{
			name:         "multiple local sources upload",
			sources:      []PathInfo{local("a.txt"), local("b.txt")},
			dest:         remote("inst1", "/home/ubuntu/"),
			wantDir:      "upload",
			wantInstance: "inst1",
		},
		{
			name:         "multiple remote sources same instance download",
			sources:      []PathInfo{remote("inst1", "/a.txt"), remote("inst1", "/b.txt")},
			dest:         local("./"),
			wantDir:      "download",
			wantInstance: "inst1",
		},
		{
			name:          "remote to remote rejected",
			sources:       []PathInfo{remote("inst1", "/a.txt")},
			dest:          remote("inst2", "/b.txt"),
			expectError:   true,
			errorContains: "cannot transfer from remote to remote",
		},
		{
			name:          "multiple different remote instances rejected",
			sources:       []PathInfo{remote("inst1", "/a.txt"), remote("inst2", "/b.txt")},
			dest:          local("./"),
			expectError:   true,
			errorContains: "cannot transfer between multiple instances",
		},
		{
			name:          "no remote path at all rejected",
			sources:       []PathInfo{local("a.txt")},
			dest:          local("b.txt"),
			expectError:   true,
			errorContains: "no remote path specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, instID, err := determineTransferDirection(tt.sources, tt.dest)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantDir, dir)
				assert.Equal(t, tt.wantInstance, instID)
			}
		})
	}
}

func TestIsValidInstanceID(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"0", true},
		{"myinstance", true},
		{"MAVERICK", true},
		{"inst-123", true},
		{"", false},                      // empty
		{"a/b", false},                   // slash
		{`a\b`, false},                   // backslash
		{"file.txt", false},              // dot
		{"123456789012345678901", false},  // >20 chars
		{"12345678901234567890", true},    // exactly 20 chars
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidInstanceID(tt.input))
		})
	}
}
