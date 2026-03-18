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

var noWait bool

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List and monitor Thunder Compute instances",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunStatus()
	},
}

func init() {
	statusCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderStatusHelp(cmd)
	})

	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVar(&noWait, "no-wait", false, "Display status once and exit without monitoring")
}

func RunStatus() error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}
	monitoring := !noWait

	if monitoring {
		if !termx.IsTerminal(os.Stdout.Fd()) {
			return fmt.Errorf("error running status TUI: not a TTY")
		}
	}

	var instances []api.Instance
	if err := tui.RunWithBusySpinner("Fetching instances...", os.Stdout, func() error {
		var e error
		instances, e = client.ListInstances()
		return e
	}); err != nil {
		return fmt.Errorf("failed to fetch instances: %w", err)
	}

	return tui.RunStatus(client, monitoring, instances)
}
