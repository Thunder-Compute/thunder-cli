package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/Thunder-Compute/thunder-cli/internal/updatepolicy"
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

		fmt.Printf("Current version: %s\n", displayVersion(currentVersion))
		ctx := context.Background()
		fmt.Println("Checking for updates...")
		res, err := updatepolicy.Check(ctx, currentVersion, true /*force*/)
		if err != nil {
			return fmt.Errorf("update policy check failed: %w", err)
		}
		if !res.Mandatory && !res.Optional {
			fmt.Printf("Already up to date (version %s)\n", displayVersion(currentVersion))
			return nil
		}

		if res.Mandatory {
			fmt.Printf("Mandatory update required: %s → %s\n", displayVersion(currentVersion), displayVersion(res.MinVersion))
		} else {
			fmt.Printf("Update available: %s → %s\n", displayVersion(currentVersion), displayVersion(res.LatestVersion))
		}

		fmt.Println("Downloading update...")
		if err := runSelfUpdate(ctx, res, useSudo); err != nil {
			// Point users to GitHub releases for manual download
			tag := res.LatestTag
			if tag == "" {
				tag = res.LatestVersion
			}
			tag = strings.TrimSpace(tag)
			if tag != "" && !strings.HasPrefix(tag, "v") && !strings.HasPrefix(tag, "V") {
				tag = "v" + tag
			}
			if tag != "" {
				fmt.Printf("You can download the latest version from GitHub: https://github.com/Thunder-Compute/thunder-cli/releases/tag/%s\n", tag)
			} else {
				fmt.Println("You can download the latest version from GitHub: https://github.com/Thunder-Compute/thunder-cli/releases")
			}
			return fmt.Errorf("self-update failed: %w", err)
		}

		if err := updatepolicy.WriteOptionalUpdateAttempt(time.Now()); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to record update timestamp: %v\n", err)
		}
		fmt.Println("Restart the CLI to use the new version")
		return nil
	},
}

func init() {
	selfUpdateCmd.Flags().String("channel", "stable", "update channel (stable or beta)")
	selfUpdateCmd.Flags().Bool("use-sudo", false, "use sudo for updating binaries in system directories")
	rootCmd.AddCommand(selfUpdateCmd)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}
