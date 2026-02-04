package cmd

import (
	"os"
	"testing"

	"github.com/Thunder-Compute/thunder-cli/internal/updatepolicy"
	"github.com/Thunder-Compute/thunder-cli/tui"
	"github.com/stretchr/testify/assert"
)

// TestRunUpdateCommand_SelfUpdateDisabled verifies that the update command
// returns an error when TNR_NO_SELFUPDATE environment variable is set.
func TestRunUpdateCommand_SelfUpdateDisabled(t *testing.T) {
	originalEnv := os.Getenv("TNR_NO_SELFUPDATE")
	defer os.Setenv("TNR_NO_SELFUPDATE", originalEnv)

	os.Setenv("TNR_NO_SELFUPDATE", "1")

	err := runUpdateCommand()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "self-update is disabled")
	assert.Contains(t, err.Error(), "TNR_NO_SELFUPDATE=1")
}

// TestHandlePMUpdate_Homebrew verifies that handlePMUpdate prints correct
// instructions for Homebrew-managed installations.
func TestHandlePMUpdate_Homebrew(t *testing.T) {
	res := updatepolicy.Result{
		CurrentVersion: "1.0.0",
		LatestVersion:  "2.0.0",
	}
	binPath := "/opt/homebrew/Cellar/tnr/1.0.0/bin/tnr"

	err := handlePMUpdate(res, binPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "package manager")
}

// TestHandlePMUpdate_Scoop verifies that handlePMUpdate prints correct
// instructions for Scoop-managed installations.
func TestHandlePMUpdate_Scoop(t *testing.T) {
	res := updatepolicy.Result{
		CurrentVersion: "1.0.0",
		LatestVersion:  "2.0.0",
	}
	binPath := "C:\\Users\\test\\scoop\\apps\\tnr\\current\\tnr.exe"

	err := handlePMUpdate(res, binPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "package manager")
}

// TestHandlePMUpdate_Winget verifies that handlePMUpdate prints correct
// instructions for winget-managed installations.
func TestHandlePMUpdate_Winget(t *testing.T) {
	res := updatepolicy.Result{
		CurrentVersion: "1.0.0",
		LatestVersion:  "2.0.0",
	}
	binPath := "C:\\Program Files\\WindowsApps\\Thunder.tnr\\tnr.exe"

	err := handlePMUpdate(res, binPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "package manager")
}

// TestHandlePMUpdate_Unknown verifies that handlePMUpdate prints generic
// instructions for unknown package manager installations.
func TestHandlePMUpdate_Unknown(t *testing.T) {
	res := updatepolicy.Result{
		CurrentVersion: "1.0.0",
		LatestVersion:  "2.0.0",
	}
	// This path doesn't match any known package manager but isPMManaged would return false
	// for this path, so we just verify the error message format
	binPath := "/some/unknown/pm/path/tnr"

	err := handlePMUpdate(res, binPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "package manager")
}

// TestUpdateCommandAnnotation verifies that the update command has the
// skipUpdateCheck annotation set to prevent redundant update checks.
func TestUpdateCommandAnnotation(t *testing.T) {
	annotation, exists := updateCmd.Annotations["skipUpdateCheck"]
	assert.True(t, exists, "updateCmd should have skipUpdateCheck annotation")
	assert.Equal(t, "true", annotation)
}

// TestUpdateCommandRegistered verifies that the update command is properly
// registered with the root command.
func TestUpdateCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "update" {
			found = true
			break
		}
	}
	assert.True(t, found, "update command should be registered with rootCmd")
}

// TestUpdateCommandHelp verifies that the update command has proper help text.
func TestUpdateCommandHelp(t *testing.T) {
	assert.Equal(t, "update", updateCmd.Use)
	assert.NotEmpty(t, updateCmd.Short)
	assert.Equal(t, "Update tnr to the latest version", updateCmd.Short)
}

// TestIsPMManaged verifies detection of package manager managed installations.
func TestIsPMManaged(t *testing.T) {
	tests := []struct {
		name     string
		binPath  string
		expected bool
	}{
		{
			name:     "Homebrew ARM path",
			binPath:  "/opt/homebrew/bin/tnr",
			expected: true,
		},
		{
			name:     "Homebrew Intel path",
			binPath:  "/usr/local/Cellar/tnr/1.0.0/bin/tnr",
			expected: true,
		},
		{
			name:     "Scoop path",
			binPath:  "C:\\Users\\test\\scoop\\apps\\tnr\\current\\tnr.exe",
			expected: true,
		},
		{
			name:     "WindowsApps path",
			binPath:  "C:\\Program Files\\WindowsApps\\Thunder.tnr\\tnr.exe",
			expected: true,
		},
		{
			name:     "Manual install path",
			binPath:  "/usr/local/bin/tnr",
			expected: false,
		},
		{
			name:     "Home directory path",
			binPath:  "/home/user/.local/bin/tnr",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPMManaged(tt.binPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDetectPackageManager verifies detection of specific package managers.
func TestDetectPackageManager(t *testing.T) {
	tests := []struct {
		name     string
		binPath  string
		expected string
	}{
		{
			name:     "Homebrew ARM",
			binPath:  "/opt/homebrew/bin/tnr",
			expected: "homebrew",
		},
		{
			name:     "Homebrew Intel",
			binPath:  "/usr/local/Cellar/tnr/1.0.0/bin/tnr",
			expected: "homebrew",
		},
		{
			name:     "Scoop",
			binPath:  "C:\\Users\\test\\scoop\\apps\\tnr\\current\\tnr.exe",
			expected: "scoop",
		},
		{
			name:     "WindowsApps (winget)",
			binPath:  "C:\\Program Files\\WindowsApps\\Thunder.tnr\\tnr.exe",
			expected: "winget",
		},
		{
			name:     "Unknown",
			binPath:  "/usr/local/bin/tnr",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectPackageManager(tt.binPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTUIRenderUpToDate verifies that the TUI renders up-to-date message correctly.
func TestTUIRenderUpToDate(t *testing.T) {
	result := tui.RenderUpToDate("v1.0.0")
	assert.Contains(t, result, "up-to-date")
	assert.Contains(t, result, "v1.0.0")
}

// TestTUIRenderUpdateAvailable verifies that the TUI renders update available message correctly.
func TestTUIRenderUpdateAvailable(t *testing.T) {
	result := tui.RenderUpdateAvailable("v1.0.0", "v2.0.0")
	assert.Contains(t, result, "Update available")
	assert.Contains(t, result, "v1.0.0")
	assert.Contains(t, result, "v2.0.0")
}

// TestTUIRenderPMInstructions verifies that the TUI renders PM instructions correctly.
func TestTUIRenderPMInstructions(t *testing.T) {
	tests := []struct {
		name            string
		pm              string
		expectedPM      string
		expectedCommand string
	}{
		{
			name:            "Homebrew",
			pm:              "homebrew",
			expectedPM:      "Homebrew",
			expectedCommand: "brew update && brew upgrade tnr",
		},
		{
			name:            "Scoop",
			pm:              "scoop",
			expectedPM:      "Scoop",
			expectedCommand: "scoop update tnr",
		},
		{
			name:            "Winget",
			pm:              "winget",
			expectedPM:      "Windows Package Manager",
			expectedCommand: "winget upgrade Thunder.tnr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tui.RenderPMInstructions(tt.pm, "v1.0.0", "v2.0.0")
			assert.Contains(t, result, tt.expectedPM)
			assert.Contains(t, result, tt.expectedCommand)
			assert.Contains(t, result, "v1.0.0")
			assert.Contains(t, result, "v2.0.0")
		})
	}
}

// TestTUIRenderUpdateSuccess verifies that the TUI renders success message correctly.
func TestTUIRenderUpdateSuccess(t *testing.T) {
	result := tui.RenderUpdateSuccess()
	assert.Contains(t, result, "Update completed successfully")
}

// TestTUIRenderUpdateStaged verifies that the TUI renders staged message correctly.
func TestTUIRenderUpdateStaged(t *testing.T) {
	result := tui.RenderUpdateStaged()
	assert.Contains(t, result, "staged successfully")
}
