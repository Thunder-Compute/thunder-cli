package cmd

import (
	"fmt"
	"os"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [instance_id]",
	Short: "Stop a running Thunder Compute instance",
	Args:  cobra.MaximumNArgs(1),
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
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token, config.APIURL)

	var instanceID string
	var selectedInstance *api.Instance

	if len(args) == 0 {
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

		selectedInstance, err = tui.RunStopInteractive(client, instances)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled stop process")
				return nil
			}
			return err
		}
		instanceID = selectedInstance.ID
	} else {
		instanceID = args[0]

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

	if selectedInstance.Status == "STOPPING" {
		return fmt.Errorf("instance '%s' is already being stopped", instanceID)
	}

	if selectedInstance.Status == "STOPPED" {
		return fmt.Errorf("instance '%s' is already stopped", instanceID)
	}

	successMsg, err := tui.RunStopProgress(client, instanceID)
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w\n\nPossible reasons:\n• Instance may not be in a running state\n• Server error occurred\n\nTry running 'tnr status' to check the instance state", err)
	}

	if successMsg != "" {
		PrintSuccessSimple(successMsg)
	}

	return nil
}
