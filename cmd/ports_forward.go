package cmd

import (
	"fmt"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

var (
	addPortsFlag    string
	removePortsFlag string
)

// portsForwardCmd represents the ports forward command
var portsForwardCmd = &cobra.Command{
	Use:     "forward [instance]",
	Aliases: []string{"fwd"},
	Short:   "Forward HTTP ports for an instance",
	Long: `Forward HTTP ports to make services accessible.

Examples:
  tnr ports forward              # Interactive mode
  tnr ports forward 1 --add 8080,3000
  tnr ports forward 1 --add 9000-9010
  tnr ports fwd 1 --add 8080 --remove 443`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPortsForward(cmd, args); err != nil {
			PrintError(err)
			os.Exit(1)
		}
	},
}

func init() {
	portsForwardCmd.Flags().StringVar(&addPortsFlag, "add", "", "Ports to add (comma-separated or ranges like 8000-8005)")
	portsForwardCmd.Flags().StringVar(&removePortsFlag, "remove", "", "Ports to remove (comma-separated or ranges like 8000-8005)")
	portsForwardCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderPortsForwardHelp(cmd)
	})
	portsCmd.AddCommand(portsForwardCmd)
}

func runPortsForward(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	// Fetch instances
	var instances []api.Instance
	if err := tui.RunWithBusySpinner("Fetching instances...", os.Stdout, func() error {
		var e error
		instances, e = client.ListInstances()
		return e
	}); err != nil {
		return fmt.Errorf("failed to fetch instances: %w", err)
	}

	if len(instances) == 0 {
		PrintWarningSimple("No instances found. Use 'tnr create' to create a Thunder Compute instance.")
		return nil
	}

	// Determine if interactive mode (no instance specified and no flags)
	isInteractive := len(args) == 0 && addPortsFlag == "" && removePortsFlag == ""

	var selectedInstance *api.Instance

	if isInteractive {
		// Run interactive mode
		return tui.RunPortsForwardInteractive(client, instances)
	}

	// Flag mode requires instance ID
	if len(args) == 0 {
		return fmt.Errorf("instance ID required when using flags")
	}

	instanceIdentifier := args[0]

	// Find instance by ID, UUID, or Name
	selectedInstance = findInstance(instances, instanceIdentifier)

	// If not found and it's a number, try as array index
	if selectedInstance == nil {
		if index, err := strconv.Atoi(instanceIdentifier); err == nil {
			if index >= 0 && index < len(instances) {
				selectedInstance = &instances[index]
			}
		}
	}

	if selectedInstance == nil {
		return fmt.Errorf("instance '%s' not found", instanceIdentifier)
	}

	// Parse ports from flags
	add, err := utils.ParsePorts(addPortsFlag)
	if err != nil {
		return fmt.Errorf("invalid --add ports: %w", err)
	}

	remove, err := utils.ParsePorts(removePortsFlag)
	if err != nil {
		return fmt.Errorf("invalid --remove ports: %w", err)
	}

	if len(add) == 0 && len(remove) == 0 {
		return fmt.Errorf("must specify --add or --remove ports")
	}

	req := api.InstanceModifyRequest{
		AddPorts:    add,
		RemovePorts: remove,
	}

	// Make API call with progress spinner
	var portsResp *api.InstanceModifyResponse
	p := tea.NewProgram(tui.NewProgressModel("Updating ports...",
		portsForwardApiCall(client, selectedInstance.ID, req, &portsResp),
		renderPortsForwardSuccess(&portsResp),
	))
	finalModel, err := p.Run()
	if err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("operation", "ports_forward_tui")
			sentry.CaptureException(err)
		})
		return fmt.Errorf("error during port update: %w", err)
	}

	result := finalModel.(tui.ProgressModel)

	if result.Cancelled() {
		PrintWarningSimple("User cancelled port update")
		return nil
	}

	if result.Err() != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("operation", "ports_forward")
			sentry.CaptureException(result.Err())
		})
		return fmt.Errorf("failed to update ports: %w", result.Err())
	}

	return nil
}

func portsForwardApiCall(client *api.Client, instanceID string, req api.InstanceModifyRequest, resp **api.InstanceModifyResponse) tea.Cmd {
	return func() tea.Msg {
		r, err := client.ModifyInstance(instanceID, req)
		if err == nil {
			*resp = r
		}
		return tui.ProgressResultMsg{Err: err}
	}
}

func renderPortsForwardSuccess(resp **api.InstanceModifyResponse) func() string {
	return func() string {
		headerStyle := theme.Primary().Bold(true)
		labelStyle := theme.Neutral()
		valueStyle := lipgloss.NewStyle().Bold(true)
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(theme.PrimaryColor)).
			Padding(1, 2)

		var lines []string
		successTitleStyle := theme.Success()
		lines = append(lines, successTitleStyle.Render("✓ Ports updated successfully!"))
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render("Instance ID:")+" "+valueStyle.Render((*resp).Identifier))
		lines = append(lines, labelStyle.Render("Instance UUID:")+" "+valueStyle.Render((*resp).InstanceName))

		if len((*resp).HTTPPorts) > 0 {
			lines = append(lines, labelStyle.Render("Forwarded Ports:")+" "+valueStyle.Render(utils.FormatPorts((*resp).HTTPPorts)))
		} else {
			lines = append(lines, labelStyle.Render("Forwarded Ports:")+" "+valueStyle.Render("(none)"))
		}

		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Access your services:"))
		if len((*resp).HTTPPorts) > 0 {
			lines = append(lines, labelStyle.Render(fmt.Sprintf("  https://%s-<port>.thundercompute.net", (*resp).InstanceName)))
		}
		lines = append(lines, labelStyle.Render("  • Run 'tnr ports list' to see all forwarded ports"))

		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return "\n" + boxStyle.Render(content) + "\n\n"
	}
}
