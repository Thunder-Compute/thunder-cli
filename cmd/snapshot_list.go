package cmd

import (
	"fmt"
	"os"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	tea "github.com/charmbracelet/bubbletea"
	termx "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
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
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token, config.APIURL)
	monitoring := !snapshotNoWait

	if monitoring {
		if !termx.IsTerminal(os.Stdout.Fd()) {
			return fmt.Errorf("error running snapshot list TUI: not a TTY")
		}
	}

	// Show busy spinner while fetching snapshots
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
