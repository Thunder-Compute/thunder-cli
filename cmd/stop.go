package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
)

var stopCmd = &cobra.Command{
	Use:    "stop [instance_id]",
	Short:  "Stop a running Thunder Compute instance",
	Hidden: true, // TODO: Remove when stop/start is ready for public release
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStop(args)
	},
}

func init() {
	stopCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderStopHelp(cmd)
	})

	rootCmd.AddCommand(stopCmd)
}

func runStop(args []string) error {
	// TODO: Remove when stop/start is ready for public release
	return fmt.Errorf("stop is not yet available. Stay tuned!")

	config, err := LoadConfig()
	if err != nil || config.Token == "" {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	client := api.NewClient(config.Token, config.APIURL)

	// Fetch instances
	busy := tui.NewBusyModel("Fetching instances...")
	bp := tea.NewProgram(busy, tea.WithOutput(os.Stdout))
	busyDone := make(chan struct{})
	go func() {
		_, _ = bp.Run()
		close(busyDone)
	}()

	instances, err := client.ListInstances()
	bp.Send(tui.BusyDoneMsg{})
	<-busyDone

	if err != nil {
		return fmt.Errorf("failed to fetch instances: %w", err)
	}

	if len(instances) == 0 {
		PrintWarningSimple("No instances found. Use 'tnr create' to create a Thunder Compute instance.")
		return nil
	}

	// Determine which instance to stop
	var selectedInstance *api.Instance
	if len(args) == 0 {
		selectedInstance, err = tui.RunStopInteractive(client, instances)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled stop process")
				return nil
			}
			return err
		}
	} else {
		instanceID := args[0]
		for i := range instances {
			if instances[i].ID == instanceID || instances[i].UUID == instanceID {
				selectedInstance = &instances[i]
				break
			}
		}
		if selectedInstance == nil {
			return fmt.Errorf("instance '%s' not found", instanceID)
		}
	}

	// Validate instance state
	if selectedInstance.Status == "STOPPED" {
		return fmt.Errorf("instance '%s' is already stopped", selectedInstance.ID)
	}

	successMsg, err := tui.RunStopProgress(client, selectedInstance.ID)
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	if successMsg != "" {
		PrintSuccessSimple(successMsg)
	}

	return nil
}
