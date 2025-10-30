/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	termx "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var noWait bool

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List and monitor Thunder Compute instances",
	Long: `List Thunder Compute instances and their current status.

By default, continuously monitors and refreshes instance statuses.
Press q to cancel.

Use --no-wait to display status once and exit immediately.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runStatus(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	statusCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderStatusHelp(cmd)
	})

	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVar(&noWait, "no-wait", false, "Display status once and exit without monitoring")
}

func runStatus() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token)
	monitoring := !noWait

	if monitoring {
		if !termx.IsTerminal(os.Stdout.Fd()) {
			return fmt.Errorf("error running status TUI: not a TTY")
		}
	}

	return tui.RunStatus(client, monitoring, nil)
}
