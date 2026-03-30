package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
)

var snapshotListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSnapshotList()
	},
}
var snapshotNoWait bool

func init() {
	snapshotListCmd.SetHelpFunc(wrapHelp(helpmenus.RenderSnapshotListHelp))

	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotListCmd.Flags().BoolVar(&snapshotNoWait, "no-wait", false, "Display snapshots once and exit without monitoring")
}

func runSnapshotList() error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}
	monitoring := !snapshotNoWait
	interactive := tui.IsInteractive() && !JSONOutput

	// Auto-disable monitoring in non-interactive mode
	if monitoring && !interactive {
		monitoring = false
	}

	var snapshots []api.Snapshot
	if err := tui.RunWithBusySpinner("Fetching snapshots...", os.Stdout, func() error {
		var e error
		snapshots, e = client.ListSnapshots()
		return e
	}); err != nil {
		return fmt.Errorf("failed to fetch snapshots: %w", err)
	}

	if JSONOutput {
		printJSON(snapshots)
		return nil
	}

	if !interactive {
		renderPlainSnapshotTable(snapshots)
		return nil
	}

	return tui.RunSnapshotList(client, monitoring, snapshots)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
