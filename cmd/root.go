/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Thunder-Compute/thunder-cli/internal/autoupdate"
	"github.com/Thunder-Compute/thunder-cli/internal/updatepolicy"
	"github.com/Thunder-Compute/thunder-cli/internal/version"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "tnr",
	Short:         "Thunder Compute CLI",
	Long:          "tnr is the command-line interface for Thunder Compute.\nUse it to manage and connect to your Thunder Compute instances.",
	Version:       version.BuildVersion,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		tui.RenderCustomHelp(cmd)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}
}

func init() {
	tui.InitCommonStyles(os.Stdout)

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		checkIfUpdateNeeded(cmd)
		return nil
	}

	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderRootHelp(cmd)
	})

	completionCmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Generate the autocompletion script for tnr for the specified shell",
		Long: `Generate the autocompletion script for tnr for the specified shell.
See each sub-command's help for details on how to use the generated script.`,
		Run: func(cmd *cobra.Command, args []string) {
			rootCmd.GenBashCompletionV2(os.Stdout, true)
		},
	}

	completionCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderCompletionHelp(cmd)
	})

	completionCmd.AddCommand(&cobra.Command{
		Use:   "bash",
		Short: "Generate the autocompletion script for bash",
		Run: func(cmd *cobra.Command, args []string) {
			rootCmd.GenBashCompletionV2(os.Stdout, true)
		},
	})

	completionCmd.AddCommand(&cobra.Command{
		Use:   "zsh",
		Short: "Generate the autocompletion script for zsh",
		Run: func(cmd *cobra.Command, args []string) {
			rootCmd.GenZshCompletion(os.Stdout)
		},
	})

	completionCmd.AddCommand(&cobra.Command{
		Use:   "fish",
		Short: "Generate the autocompletion script for fish",
		Run: func(cmd *cobra.Command, args []string) {
			rootCmd.GenFishCompletion(os.Stdout, true)
		},
	})

	completionCmd.AddCommand(&cobra.Command{
		Use:   "powershell",
		Short: "Generate the autocompletion script for powershell",
		Run: func(cmd *cobra.Command, args []string) {
			rootCmd.GenPowerShellCompletion(os.Stdout)
		},
	})

	rootCmd.AddCommand(completionCmd)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.thunder-cli-draft.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func checkIfUpdateNeeded(cmd *cobra.Command) {
	if shouldSkipUpdateCheck(cmd) {
		return
	}

	if os.Getenv("TNR_NO_SELFUPDATE") == "1" {
		return
	}

	ctx := context.Background()

	// Apply any previously staged Windows update before checking again.
	if err := autoupdate.FinalizeStagedWindowsUpdate(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to finalize staged Windows update: %v\n", err)
	}

	policyResult, err := updatepolicy.Check(ctx, version.BuildVersion, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: update check failed: %v\n", err)
		return
	}

	if !policyResult.Mandatory && !policyResult.Optional {
		return
	}

	if policyResult.Mandatory {
		handleMandatoryUpdate(ctx, policyResult)
		return
	}

	handleOptionalUpdate(ctx, policyResult)
}

func handleMandatoryUpdate(parentCtx context.Context, res updatepolicy.Result) {
	displayCurrent := displayVersion(res.CurrentVersion)
	displayMin := displayVersion(res.MinVersion)
	fmt.Fprintf(os.Stderr, "⚠ Mandatory update required: current %s, minimum %s.\n", displayCurrent, displayMin)

	binPath, _ := getCurrentBinaryPath()
	if binPath != "" && isPMManaged(binPath) {
		fmt.Fprintln(os.Stderr, "This installation is managed by a package manager. Please upgrade using brew/winget/scoop or reinstall the latest release.")
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "Attempting automatic update...")
	updateCtx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	if err := runSelfUpdate(updateCtx, res); err != nil {
		fmt.Fprintf(os.Stderr, "Automatic update failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "Download the latest version from GitHub: https://github.com/Thunder-Compute/thunder-cli/releases/tag/%s and reinstall the CLI.\n", releaseTag(res))
		os.Exit(1)
	}

	// Try to finalize the update immediately if possible (Windows only, when already elevated or writable)
	binPath, _ = getCurrentBinaryPath()
	if binPath != "" {
		shouldReexec, err := autoupdate.TryFinalizeStagedUpdateImmediately(parentCtx, binPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Update staged successfully. Please re-run your command to complete the update.")
			os.Exit(0)
		}
		if shouldReexec {
			fmt.Fprintln(os.Stderr, "Update completed successfully. Please re-run your command.")
			os.Exit(0)
		}
	}
	
	fmt.Fprintln(os.Stderr, "Update staged successfully. Please re-run your command to complete the update.")
	os.Exit(0)
}

func handleOptionalUpdate(parentCtx context.Context, res updatepolicy.Result) {
	binPath, _ := getCurrentBinaryPath()
	if binPath != "" && isPMManaged(binPath) {
		fmt.Printf("⚠ Update available: %s → %s. Update via your package manager (e.g. brew upgrade tnr).\n",
			displayVersion(res.CurrentVersion), displayVersion(res.LatestVersion))
		return
	}

	lastAttempt, err := updatepolicy.ReadOptionalUpdateAttempt()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: unable to read optional update cache: %v\n", err)
	}
	if !lastAttempt.IsZero() && time.Since(lastAttempt) < updatepolicy.OptionalUpdateTTL {
		fmt.Printf("ℹ️  Update available: %s → %s. Automatic update skipped (last attempt %s). Reinstall from the latest release to update now.\n",
			displayVersion(res.CurrentVersion), displayVersion(res.LatestVersion), lastAttempt.Format(time.RFC1123))
		return
	}

	fmt.Printf("Automatically updating to %s. Please wait...\n", displayVersion(res.LatestVersion))

	attemptTime := time.Now()
	updateCtx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	updateErr := runSelfUpdate(updateCtx, res)
	if writeErr := updatepolicy.WriteOptionalUpdateAttempt(attemptTime); writeErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to record optional update attempt: %v\n", writeErr)
	}

	if updateErr == nil {
		fmt.Println("Update finished! You can now re-run your command.")
	} else {
		fmt.Fprintf(os.Stderr, "Warning: optional update failed: %v\n", updateErr)
		fmt.Printf("You can download the latest version from GitHub: https://github.com/Thunder-Compute/thunder-cli/releases/tag/%s and reinstall the CLI.\n", releaseTag(res))
	}
	os.Exit(0)
}

func runSelfUpdate(ctx context.Context, res updatepolicy.Result) error {
	source := autoupdate.Source{
		Version:     res.LatestVersion,
		ReleaseTag:  releaseTag(res),
		AssetURL:    res.AssetURL,
		Checksum:    res.ExpectedSHA256,
		ChecksumURL: res.ChecksumURL,
	}
	return autoupdate.PerformUpdate(ctx, source)
}

func releaseTag(res updatepolicy.Result) string {
	tag := res.LatestTag
	if tag == "" {
		tag = res.LatestVersion
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	if !strings.HasPrefix(tag, "v") && !strings.HasPrefix(tag, "V") {
		tag = "v" + tag
	}
	return tag
}

func displayVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		return v
	}
	return "v" + v
}


func shouldSkipUpdateCheck(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}

	for current := cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "help", "completion", "version":
			return true
		}

		if current.Annotations != nil && current.Annotations["skipUpdateCheck"] == "true" {
			return true
		}
	}

	if helpFlag := cmd.Flags().Lookup("help"); helpFlag != nil && helpFlag.Changed {
		return true
	}

	return false
}
