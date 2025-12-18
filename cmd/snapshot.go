package cmd

import (
	"github.com/spf13/cobra"
)

// snapshotCmd represents the snapshot parent command
var snapshotCmd = &cobra.Command{
	Use:     "snapshot",
	Aliases: []string{"snapshots"},
	Short:   "Manage Thunder Compute snapshots",
	Long:    "Create snapshots of your Thunder Compute instances.",
	Run: func(cmd *cobra.Command, args []string) {
		// Show help when parent command is called without subcommand
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
