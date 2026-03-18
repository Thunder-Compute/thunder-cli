package cmd

import (
	"fmt"
	"os"

	termx "github.com/charmbracelet/x/term"
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
	snapshotListCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSnapshotListHelp(cmd)
	})

	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotListCmd.Flags().BoolVar(&snapshotNoWait, "no-wait", false, "Display snapshots once and exit without monitoring")
}

func runSnapshotList() error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}
	monitoring := !snapshotNoWait

	if monitoring {
		if !termx.IsTerminal(os.Stdout.Fd()) {
			return fmt.Errorf("error running snapshot list TUI: not a TTY")
		}
	}

	var snapshots []api.Snapshot
	if err := tui.RunWithBusySpinner("Fetching snapshots...", os.Stdout, func() error {
		var e error
		snapshots, e = client.ListSnapshots()
		return e
	}); err != nil {
		return fmt.Errorf("failed to fetch snapshots: %w", err)
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
