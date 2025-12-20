package cmd

import (
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"

	"github.com/spf13/cobra"
)

// snapshotCmd represents the snapshot parent command
var snapshotCmd = &cobra.Command{
	Use:     "snapshot",
	Aliases: []string{"snapshots", "snap"},
	Short:   "Manage Thunder Compute snapshots",
	Long:    "Create snapshots of your Thunder Compute instances.",
	Run: func(cmd *cobra.Command, args []string) {
		// Show help when parent command is called without subcommand
		_ = cmd.Help()
	},
}

func init() {
	snapshotCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSnapshotHelp(cmd)
	})
	rootCmd.AddCommand(snapshotCmd)
}
