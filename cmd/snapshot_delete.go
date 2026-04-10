package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
)

var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete [snapshot_name]",
	Short: "Delete a snapshot",
	Args:  wrapArgs(cobra.MaximumNArgs(1)),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSnapshotDelete(args)
	},
}

func init() {
	snapshotDeleteCmd.SetHelpFunc(wrapHelp(helpmenus.RenderSnapshotDeleteHelp))

	snapshotCmd.AddCommand(snapshotDeleteCmd)
}

func runSnapshotDelete(args []string) error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	interactive := tui.IsInteractive() && !JSONOutput

	var snapshotID string
	var selectedSnapshot *api.Snapshot

	if len(args) == 0 {
		if !interactive {
			return usageErr("snapshot name or ID required in non-interactive mode")
		}
		// Interactive mode: fetch snapshots and let user select
		var snapshots []api.Snapshot
		if err := tui.RunWithBusySpinner("Fetching snapshots...", os.Stdout, func() error {
			var e error
			snapshots, e = client.ListSnapshots()
			return e
		}); err != nil {
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
			if errors.Is(err, tui.ErrCancelled) {
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
		var snapshots []api.Snapshot
		if err := tui.RunWithBusySpinner("Validating snapshot...", os.Stdout, func() error {
			var e error
			snapshots, e = client.ListSnapshots()
			return e
		}); err != nil {
			return fmt.Errorf("failed to fetch snapshots: %w", err)
		}

		for i := range snapshots {
			if snapshots[i].Name == snapshotName || snapshots[i].ID == snapshotName {
				selectedSnapshot = &snapshots[i]
				break
			}
		}

		if selectedSnapshot == nil {
			return usageErr("snapshot '%s' not found", snapshotName)
		}

		snapshotID = selectedSnapshot.ID

		// Confirm deletion (skip with --yes)
		if !YesFlag {
			if !interactive {
				return usageErr("use --yes to confirm deletion in non-interactive mode")
			}
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
	}

	if !interactive {
		// Non-interactive: direct API call
		fmt.Fprintln(os.Stderr, "Deleting snapshot...")
		if deleteErr := client.DeleteSnapshot(snapshotID); deleteErr != nil {
			return fmt.Errorf("failed to delete snapshot: %w", deleteErr)
		}
		if JSONOutput {
			printJSON(map[string]string{"snapshot": selectedSnapshot.Name, "status": "deleted"})
		} else {
			fmt.Printf("Deleted snapshot '%s'\n", selectedSnapshot.Name)
		}
		return nil
	}

	// Run deletion with progress
	successMsg, err := tui.RunSnapshotDeleteProgress(client, snapshotID, selectedSnapshot.Name)
	if err != nil {
		if !isUserError(err) {
			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetTag("operation", "snapshot_delete")
				sentry.CaptureException(err)
			})
		}
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	if successMsg != "" {
		PrintSuccessSimple(successMsg)
	}

	return nil
}
