package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete [snapshot_name]",
	Short: "Delete a snapshot",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSnapshotDelete(args)
	},
}

func init() {
	snapshotDeleteCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSnapshotDeleteHelp(cmd)
	})

	snapshotCmd.AddCommand(snapshotDeleteCmd)
}

func runSnapshotDelete(args []string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token, config.APIURL)

	var snapshotID string
	var selectedSnapshot *api.Snapshot

	if len(args) == 0 {
		// Interactive mode: fetch snapshots and let user select
		busy := tui.NewBusyModel("Fetching snapshots...")
		bp := tea.NewProgram(busy, tea.WithOutput(os.Stdout))
		busyDone := make(chan struct{})
		go func() {
			_, _ = bp.Run()
			close(busyDone)
		}()

		snapshots, err := client.ListSnapshots()
		bp.Send(tui.BusyDoneMsg{})
		<-busyDone

		if err != nil {
			return fmt.Errorf("failed to fetch snapshots: %w", err)
		}

		if len(snapshots) == 0 {
			PrintWarningSimple("No snapshots found.")
			return nil
		}

		// Sort by creation time (oldest first) to match list command
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].CreatedAt < snapshots[j].CreatedAt
		})

		selectedSnapshot, err = tui.RunSnapshotDeleteInteractive(client, snapshots)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled delete process")
				return nil
			}
			return err
		}
		snapshotID = selectedSnapshot.ID
	} else {
		// Non-interactive mode: use provided snapshot name
		snapshotName := args[0]

		// Validate snapshot exists
		busy := tui.NewBusyModel("Validating snapshot...")
		bp := tea.NewProgram(busy, tea.WithOutput(os.Stdout))
		busyDone := make(chan struct{})
		go func() {
			_, _ = bp.Run()
			close(busyDone)
		}()

		snapshots, err := client.ListSnapshots()
		bp.Send(tui.BusyDoneMsg{})
		<-busyDone

		if err != nil {
			return fmt.Errorf("failed to fetch snapshots: %w", err)
		}

		for i := range snapshots {
			if snapshots[i].Name == snapshotName || snapshots[i].ID == snapshotName {
				selectedSnapshot = &snapshots[i]
				break
			}
		}

		if selectedSnapshot == nil {
			return fmt.Errorf("snapshot '%s' not found", snapshotName)
		}

		snapshotID = selectedSnapshot.ID

		// Always confirm before deletion
		fmt.Println()
		fmt.Printf("About to delete snapshot: %s\n", selectedSnapshot.Name)
		fmt.Printf("Status: %s\n", selectedSnapshot.Status)
		fmt.Printf("Disk Size: %d GB\n", selectedSnapshot.MinimumDiskSizeGB)
		fmt.Println()
		fmt.Print("Are you sure you want to delete this snapshot? (yes/no): ")

		var confirmation string
		fmt.Scanln(&confirmation)

		if confirmation != "yes" && confirmation != "y" {
			PrintWarningSimple("Deletion cancelled")
			return nil
		}
	}

	// Run deletion with progress
	successMsg, err := tui.RunSnapshotDeleteProgress(client, snapshotID, selectedSnapshot.Name)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	if successMsg != "" {
		PrintSuccessSimple(successMsg)
	}

	return nil
}
