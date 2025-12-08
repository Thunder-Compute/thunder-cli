package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Thunder-Compute/thunder-cli/internal/autoupdate"
	"github.com/Thunder-Compute/thunder-cli/internal/updatepolicy"
	"github.com/Thunder-Compute/thunder-cli/internal/version"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update tnr to the latest version",
	Annotations: map[string]string{
		"skipUpdateCheck": "true",
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runUpdateCommand(); err != nil {
			PrintError(err)
			os.Exit(1)
		}
	},
}

func init() {
	updateCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderUpdateHelp(cmd)
	})

	rootCmd.AddCommand(updateCmd)
}

func runUpdateCommand() error {
	if os.Getenv("TNR_NO_SELFUPDATE") == "1" {
		return fmt.Errorf("self-update is disabled (TNR_NO_SELFUPDATE=1)")
	}

	// Finalize any previously staged Windows update before checking again
	if err := autoupdate.FinalizeWindowsSwap(); err != nil {
		fmt.Fprintln(os.Stderr, tui.RenderWarning(fmt.Sprintf("failed to finalize staged Windows update: %v", err)))
	}

	parentCtx := context.Background()
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	// Force fresh check (ignores cache)
	policyResult, err := updatepolicy.Check(ctx, version.BuildVersion, true)
	if err != nil {
		return fmt.Errorf("update check failed: %w", err)
	}

	if !policyResult.Mandatory && !policyResult.Optional {
		fmt.Println(tui.RenderUpToDate(displayVersion(policyResult.CurrentVersion)))
		return nil
	}

	binPath, _ := getCurrentBinaryPath()
	if binPath != "" && isPMManaged(binPath) {
		return handlePMUpdate(policyResult, binPath)
	}

	if policyResult.Mandatory {
		handleMandatoryUpdate(parentCtx, policyResult)
		// handleMandatoryUpdate exits the process
		return nil
	}

	// Explicit request: skip TTL cache
	return handleExplicitOptionalUpdate(parentCtx, policyResult)
}

func handlePMUpdate(res updatepolicy.Result, binPath string) error {
	pm := detectPackageManager(binPath)
	currentVer := displayVersion(res.CurrentVersion)
	latestVer := displayVersion(res.LatestVersion)

	fmt.Println(tui.RenderPMInstructions(pm, currentVer, latestVer))

	return fmt.Errorf("cannot auto-update: installation is managed by a package manager")
}

// handleExplicitOptionalUpdate handles explicit update requests.
// Unlike automatic updates, this skips TTL caching and doesn't record attempt timestamps.
func handleExplicitOptionalUpdate(parentCtx context.Context, res updatepolicy.Result) error {
	currentVer := displayVersion(res.CurrentVersion)
	latestVer := displayVersion(res.LatestVersion)

	fmt.Println(tui.RenderUpdating(currentVer, latestVer))

	updateCtx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	if err := runSelfUpdate(updateCtx, res); err != nil {
		releaseURL := fmt.Sprintf("https://github.com/Thunder-Compute/thunder-cli/releases/tag/%s", releaseTag(res))
		fmt.Fprintln(os.Stderr, tui.RenderUpdateFailed(err, releaseURL))
		return err
	}

	// Finalize update immediately if possible (Windows only)
	binPath, _ := getCurrentBinaryPath()
	if binPath != "" {
		shouldReexec, err := autoupdate.TryFinalizeStagedUpdateImmediately(parentCtx, binPath)
		if err != nil {
			fmt.Println(tui.RenderUpdateStaged())
			return nil
		}
		if shouldReexec {
			fmt.Println(tui.RenderUpdateRerun())
			return nil
		}
	}

	fmt.Println(tui.RenderUpdateSuccess())
	return nil
}
