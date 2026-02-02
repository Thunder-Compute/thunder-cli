package cmd

import (
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"

	"github.com/spf13/cobra"
)

// portsCmd represents the ports parent command
var portsCmd = &cobra.Command{
	Use:     "ports",
	Aliases: []string{"port"},
	Short:   "Manage HTTP port forwarding for instances",
	Long:    "Commands for listing and managing forwarded HTTP ports on Thunder instances.",
	Run: func(cmd *cobra.Command, args []string) {
		// Show help when parent command is called without subcommand
		_ = cmd.Help()
	},
}

func init() {
	portsCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderPortsHelp(cmd)
	})
	rootCmd.AddCommand(portsCmd)
}
