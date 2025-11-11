/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/Thunder-Compute/thunder-cli/internal/autoupdate"
	"github.com/Thunder-Compute/thunder-cli/internal/updatecheck"
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

var updatePromptOnce sync.Once

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
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		maybePromptForUpdate(cmd)
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

func maybePromptForUpdate(cmd *cobra.Command) {
	updatePromptOnce.Do(func() {
		if shouldSkipUpdateCheck(cmd) {
			return
		}

		if os.Getenv("TNR_NO_SELFUPDATE") == "1" {
			return
		}

		ctx := context.Background()

		// Start background autoupdate if needed (non-blocking).
		autoupdate.MaybeStartBackgroundUpdate(ctx, version.BuildVersion)

		result, err := updatecheck.Check(ctx, version.BuildVersion)
		if err != nil || result.Skipped || !result.Outdated {
			return
		}

		var binPath string
		if path, err := getCurrentBinaryPath(); err == nil {
			binPath = path
		}

		if binPath != "" && isPMManaged(binPath) {
			fmt.Printf("⚠️  Update available: %s → %s. Update via your package manager (e.g. brew upgrade tnr).\n", result.CurrentVersion, result.LatestVersion)
			return
		}

		fmt.Printf("⚠️  Update available: %s → %s. Run `tnr self-update` to upgrade.\n", result.CurrentVersion, result.LatestVersion)
	})
}

func shouldSkipUpdateCheck(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}

	for current := cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "self-update", "help", "completion", "version":
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
