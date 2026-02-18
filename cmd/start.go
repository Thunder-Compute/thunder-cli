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

var startCmd = &cobra.Command{
	Use:    "start [instance_id]",
	Short:  "Start a stopped Thunder Compute instance",
	Hidden: true, // TODO: Remove when stop/start is ready for public release
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStart(args)
	},
}

func init() {
	startCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderStartHelp(cmd)
	})

	rootCmd.AddCommand(startCmd)
}

func runStart(args []string) error {
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

	// Determine which instance to start
	var selectedInstance *api.Instance
	if len(args) == 0 {
		selectedInstance, err = tui.RunStartInteractive(client, instances)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled start process")
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
	if selectedInstance.Status == "RUNNING" {
		return fmt.Errorf("instance '%s' is already running", selectedInstance.ID)
	}
	if selectedInstance.Status == "STARTING" {
		return fmt.Errorf("instance '%s' is already starting", selectedInstance.ID)
	}

	successMsg, err := tui.RunStartProgress(client, selectedInstance.ID)
	if err != nil {
		return fmt.Errorf("failed to start instance: %w\n\nPossible reasons:\n• Instance may not be in a stopped state\n• Server error occurred\n\nTry running 'tnr status' to check the instance state", err)
	}

	if successMsg != "" {
		PrintSuccessSimple(successMsg)
	}

	return nil
}
