/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/joshuawatkins04/thunder-cli-draft/api"
	"github.com/joshuawatkins04/thunder-cli-draft/tui"
	"github.com/spf13/cobra"
)

var noWait bool

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List and monitor Thunder Compute instances",
	Long: `List all Thunder Compute instances in your account with their current status.

By default, the command will continuously monitor instances that are in transition 
states (STARTING or DELETING) and automatically exit when all instances are stable.

Use the --no-wait flag to display the status once and exit immediately.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runStatus(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
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

	if err := tui.RunStatus(client, monitoring); err != nil {
		return err
	}

	return nil
}
