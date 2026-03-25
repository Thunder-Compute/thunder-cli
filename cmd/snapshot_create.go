package cmd

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
)

var (
	snapshotInstanceID string
	snapshotName       string
)

var snapshotCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a snapshot from an instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSnapshotCreate(cmd)
	},
}

func init() {
	snapshotCreateCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSnapshotCreateHelp(cmd)
	})

	snapshotCmd.AddCommand(snapshotCreateCmd)

	snapshotCreateCmd.Flags().StringVar(&snapshotInstanceID, "instance-id", "", "Instance ID or UUID to snapshot")
	snapshotCreateCmd.Flags().StringVar(&snapshotName, "name", "", "Name for the snapshot")
}

func createSnapshotCmd(client *api.Client, req api.CreateSnapshotRequest, resp **api.CreateSnapshotResponse) tea.Cmd {
	return func() tea.Msg {
		r, err := client.CreateSnapshot(req)
		if err == nil {
			*resp = r
		}
		return tui.ProgressResultMsg{Err: err}
	}
}

func renderSnapshotCreateSuccess(resp **api.CreateSnapshotResponse) func() string {
	return func() string {
		labelStyle := theme.Neutral()
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(theme.PrimaryColor)).
			Padding(1, 2)

		var lines []string
		successTitleStyle := theme.Success()
		lines = append(lines, successTitleStyle.Render("✓ Snapshot created successfully!"))
		lines = append(lines, "")
		if (*resp).Message != "" {
			lines = append(lines, labelStyle.Render("Message: ")+(*resp).Message)
		}

		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return "\n" + boxStyle.Render(content) + "\n\n"
	}
}

func runSnapshotCreate(cmd *cobra.Command) error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	isInteractive := !cmd.Flags().Changed("instance-id")

	var instanceID, name string

	if isInteractive {
		// Run interactive flow
		createConfig, err := tui.RunSnapshotCreateInteractive(client)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled snapshot creation")
				return nil
			}
			if errors.Is(err, tui.ErrNoRunningInstances) {
				PrintWarningSimple("No running instances found. Snapshots can only be created from instances in RUNNING state.")
				return nil
			}
			return err
		}
		instanceID = createConfig.InstanceID
		name = createConfig.Name
	} else {
		// Non-interactive mode: validate flags
		if snapshotInstanceID == "" {
			return fmt.Errorf("--instance-id is required")
		}
		if snapshotName == "" {
			return fmt.Errorf("--name is required")
		}
		instanceID = snapshotInstanceID
		name = snapshotName

		// Validate instance exists and is in RUNNING state
		var instances []api.Instance
		if err := tui.RunWithBusySpinner("Validating instance...", os.Stdout, func() error {
			var e error
			instances, e = client.ListInstances()
			return e
		}); err != nil {
			return fmt.Errorf("failed to fetch instances: %w", err)
		}

		foundInstance := findInstance(instances, instanceID)

		if foundInstance == nil {
			return fmt.Errorf("instance '%s' not found", instanceID)
		}

		if foundInstance.Status != "RUNNING" {
			return fmt.Errorf("instance must be in RUNNING state to create snapshot (current state: %s)", foundInstance.Status)
		}

		// Use UUID for the API call
		instanceID = foundInstance.UUID
	}

	req := api.CreateSnapshotRequest{
		InstanceID: instanceID,
		Name:       name,
	}

	interactive := tui.IsInteractive() && !JSONOutput

	var snapshotResp *api.CreateSnapshotResponse

	if !interactive {
		fmt.Fprintln(os.Stderr, "Creating snapshot...")
		snapshotResp, err = client.CreateSnapshot(req)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}
		if JSONOutput {
			printJSON(snapshotResp)
		} else {
			msg := "Snapshot created"
			if snapshotResp != nil && snapshotResp.Message != "" {
				msg = snapshotResp.Message
			}
			fmt.Println(msg)
		}
		return nil
	}

	progressModel := tui.NewProgressModel("Creating snapshot...",
		createSnapshotCmd(client, req, &snapshotResp),
		renderSnapshotCreateSuccess(&snapshotResp),
	)
	program := tea.NewProgram(progressModel)
	finalModel, runErr := program.Run()
	if runErr != nil {
		return fmt.Errorf("failed to render progress: %w", runErr)
	}

	result := finalModel.(tui.ProgressModel)

	if result.Cancelled() {
		PrintWarningSimple("User cancelled snapshot creation")
		return nil
	}

	if result.Err() != nil {
		return fmt.Errorf("failed to create snapshot: %w", result.Err())
	}

	return nil
}
