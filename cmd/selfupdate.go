package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
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

		if !userWritable(binPath) {
			return errors.New("install path is not writable by current user; re-install to ~/.tnr/bin or use your package manager")
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

		// Download and apply update
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
	rootCmd.AddCommand(selfUpdateCmd)
}
