package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
)

var noWait bool
var verboseStatus bool

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List and monitor Thunder Compute instances",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunStatus()
	},
}

func init() {
	statusCmd.SetHelpFunc(wrapHelp(helpmenus.RenderStatusHelp))

	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVar(&noWait, "no-wait", false, "Display status once and exit without monitoring")
	statusCmd.Flags().BoolVarP(&verboseStatus, "verbose", "v", false, "Show all events regardless of age")
}

func RunStatus() error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}
	monitoring := !noWait
	interactive := tui.IsInteractive() && !JSONOutput

	// Auto-disable monitoring in non-interactive mode
	if monitoring && !interactive {
		monitoring = false
	}

	var instances []api.Instance
	if err := tui.RunWithBusySpinner("Fetching instances...", os.Stdout, func() error {
		var e error
		instances, e = client.ListInstances()
		return e
	}); err != nil {
		return fmt.Errorf("failed to fetch instances: %w", err)
	}

	if JSONOutput {
		printJSON(instances)
		return nil
	}

	if !interactive {
		renderPlainStatusTable(instances, verboseStatus)
		return nil
	}

	return tui.RunStatus(client, monitoring, instances, verboseStatus)
}
