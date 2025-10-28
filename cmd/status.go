/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joshuawatkins04/thunder-cli-draft/api"
	"github.com/joshuawatkins04/thunder-cli-draft/tui"
	helpmenus "github.com/joshuawatkins04/thunder-cli-draft/tui/help-menus"
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

	busy := tui.NewBusyModel("Fetching instances...")
	bp := tea.NewProgram(busy)
	busyDone := make(chan struct{})

	go func() {
		_, _ = bp.Run()
		close(busyDone)
	}()

	instances, err := client.ListInstances()

	bp.Send(tui.BusyDoneMsg{})
	<-busyDone

	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	if err := tui.RunStatus(client, monitoring, instances); err != nil {
		return err
	}

	return nil
}
