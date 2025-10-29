package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/internal/version"
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

		channel, _ := cmd.Flags().GetString("channel")
		if channel == "" {
			channel = "stable"
		}

		// TODO: Implement self-update using the new go-selfupdate API
		fmt.Println("Self-update functionality is temporarily disabled due to API changes")
		fmt.Printf("Current version: %s\n", version.BuildVersion)
		fmt.Println("Please download the latest version manually from the releases page")
		return nil
	},
}

func init() {
	selfUpdateCmd.Flags().String("channel", "stable", "update channel (stable or beta)")
	rootCmd.AddCommand(selfUpdateCmd)
}
