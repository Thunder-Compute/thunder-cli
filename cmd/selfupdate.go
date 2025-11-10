package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/internal/version"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

func isPMManaged(binPath string) bool {
	return strings.Contains(binPath, "/opt/homebrew/") ||
		strings.Contains(binPath, "/usr/local/Cellar/") ||
		strings.Contains(binPath, "\\scoop\\apps\\") ||
		strings.Contains(binPath, "WindowsApps")
}

func userWritable(path string) bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	return strings.HasPrefix(path, u.HomeDir)
}

func getCurrentBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update tnr to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("TNR_NO_SELFUPDATE") == "1" {
			fmt.Println("Self-update disabled by TNR_NO_SELFUPDATE=1")
			return nil
		}

		binPath, err := getCurrentBinaryPath()
		if err != nil {
			return err
		}

		if isPMManaged(binPath) {
			fmt.Println("Managed by package manager; use brew upgrade tnr / scoop update tnr / winget upgrade Thunder.tnr")
			return nil
		}

		useSudo, _ := cmd.Flags().GetBool("use-sudo")
		
		if !userWritable(binPath) && !useSudo {
			fmt.Printf("⚠️  Installation path requires elevated permissions: %s\n\n", binPath)
			fmt.Println("Choose one of the following options:")
			fmt.Println()
			fmt.Println("1. Update with sudo (requires password):")
			fmt.Println("   tnr self-update --use-sudo")
			fmt.Println()
			fmt.Println("2. Reinstall to user-writable location:")
			fmt.Println("   curl -fsSL https://raw.githubusercontent.com/Thunder-Compute/thunder-cli/main/scripts/install.sh | bash")
			fmt.Println()
			fmt.Println("3. Install via Homebrew (recommended for macOS):")
			fmt.Println("   brew tap Thunder-Compute/tap && brew install tnr")
			fmt.Println()
			return errors.New("update requires elevated permissions or reinstallation")
		}

		currentVersion := version.BuildVersion
		if currentVersion == "dev" || currentVersion == "" {
			fmt.Println("Development build detected. Cannot check for updates.")
			fmt.Println("Please download the latest version manually from https://github.com/Thunder-Compute/thunder-cli/releases")
			return nil
		}

		fmt.Printf("Current version: %s\n", currentVersion)
		fmt.Println("Checking for updates...")

		// Detect the latest version from GitHub releases
		ctx := context.Background()
		slug := selfupdate.ParseSlug("Thunder-Compute/thunder-cli")
		
		// Note: We use the simple DetectLatest approach for now.
		// To add checksum validation in the future, use:
		//   updater, _ := selfupdate.NewUpdater(selfupdate.Config{
		//       Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
		//   })
		//   latest, found, err := updater.DetectLatest(ctx, slug)
		
		latest, found, err := selfupdate.DetectLatest(ctx, slug)
		if err != nil {
			return fmt.Errorf("error occurred while detecting version: %w", err)
		}

		if !found {
			return fmt.Errorf("no release found for %s/%s", runtime.GOOS, runtime.GOARCH)
		}

		// Compare versions
		if latest.LessOrEqual(currentVersion) {
			fmt.Printf("Already up to date (version %s)\n", currentVersion)
			return nil
		}

		fmt.Printf("New version available: %s\n", latest.Version())
		fmt.Println("Downloading update...")

		// Get current executable path
		exe, err := selfupdate.ExecutablePath()
		if err != nil {
			return fmt.Errorf("could not locate executable path: %w", err)
		}

		// If using sudo, download to temp location then move with sudo
		if useSudo {
			// Create temp file for download
			tmpDir := os.TempDir()
			tmpBinary := filepath.Join(tmpDir, "tnr.new")
			
			// Download to temp location
			if err := selfupdate.UpdateTo(ctx, latest.AssetURL, latest.AssetName, tmpBinary); err != nil {
				return fmt.Errorf("error occurred while downloading update: %w", err)
			}

			// Make it executable
			if err := os.Chmod(tmpBinary, 0755); err != nil {
				return fmt.Errorf("failed to set executable permissions: %w", err)
			}

			fmt.Println("Downloaded successfully. Installing with sudo (you may be prompted for your password)...")

			// Use sudo to move the file
			sudoCmd := exec.Command("sudo", "mv", tmpBinary, exe)
			sudoCmd.Stdout = os.Stdout
			sudoCmd.Stderr = os.Stderr
			sudoCmd.Stdin = os.Stdin

			if err := sudoCmd.Run(); err != nil {
				return fmt.Errorf("failed to install with sudo: %w", err)
			}

			fmt.Printf("Successfully updated to version %s\n", latest.Version())
			fmt.Println("Restart the CLI to use the new version")
			return nil
		}

		// Normal update (user-writable location)
		if err := selfupdate.UpdateTo(ctx, latest.AssetURL, latest.AssetName, exe); err != nil {
			return fmt.Errorf("error occurred while updating binary: %w", err)
		}

		fmt.Printf("Successfully updated to version %s\n", latest.Version())
		fmt.Println("Restart the CLI to use the new version")
		return nil
	},
}

func init() {
	selfUpdateCmd.Flags().String("channel", "stable", "update channel (stable or beta)")
	selfUpdateCmd.Flags().Bool("use-sudo", false, "use sudo for updating binaries in system directories")
	rootCmd.AddCommand(selfUpdateCmd)
}
