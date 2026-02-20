package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
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
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
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

	// Find instance by ID or UUID
	for i := range instances {
		if instances[i].ID == instanceIdentifier || instances[i].UUID == instanceIdentifier {
			selectedInstance = &instances[i]
			break
		}
	}

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
	p := tea.NewProgram(newPortsForwardProgressModel(client, selectedInstance.ID, req))
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error during port update: %w", err)
	}

	progressModel := finalModel.(portsForwardProgressModel)

	if progressModel.cancelled {
		PrintWarningSimple("User cancelled port update")
		return nil
	}

	if progressModel.err != nil {
		return fmt.Errorf("failed to update ports: %w", progressModel.err)
	}

	return nil
}

// Progress model for port forward operation
type portsForwardProgressModel struct {
	client     *api.Client
	instanceID string
	req        api.InstanceModifyRequest
	spinner    spinner.Model
	message    string
	done       bool
	err        error
	resp       *api.InstanceModifyResponse
	cancelled  bool
}

func newPortsForwardProgressModel(client *api.Client, instanceID string, req api.InstanceModifyRequest) portsForwardProgressModel {
	theme.Init(os.Stdout)
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.Primary()

	return portsForwardProgressModel{
		client:     client,
		instanceID: instanceID,
		req:        req,
		spinner:    s,
		message:    "Updating ports...",
	}
}

type portsForwardResultMsg struct {
	resp *api.InstanceModifyResponse
	err  error
}

func (m portsForwardProgressModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		portsForwardCmd_apiCall(m.client, m.instanceID, m.req),
	)
}

func portsForwardCmd_apiCall(client *api.Client, instanceID string, req api.InstanceModifyRequest) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.ModifyInstance(instanceID, req)
		return portsForwardResultMsg{
			resp: resp,
			err:  err,
		}
	}
}

func (m portsForwardProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cancelled = true
			return m, tea.Quit
		}

	case portsForwardResultMsg:
		m.done = true
		m.err = msg.err
		m.resp = msg.resp
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m portsForwardProgressModel) View() string {
	if m.done {
		if m.cancelled {
			return ""
		}

		if m.err != nil {
			return ""
		}

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
		lines = append(lines, labelStyle.Render("Instance ID:")+" "+valueStyle.Render(m.resp.Identifier))
		lines = append(lines, labelStyle.Render("Instance UUID:")+" "+valueStyle.Render(m.resp.InstanceName))

		if len(m.resp.HTTPPorts) > 0 {
			lines = append(lines, labelStyle.Render("Forwarded Ports:")+" "+valueStyle.Render(utils.FormatPorts(m.resp.HTTPPorts)))
		} else {
			lines = append(lines, labelStyle.Render("Forwarded Ports:")+" "+valueStyle.Render("(none)"))
		}

		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Access your services:"))
		if len(m.resp.HTTPPorts) > 0 {
			lines = append(lines, labelStyle.Render(fmt.Sprintf("  https://%s-<port>.thundercompute.net", m.resp.InstanceName)))
		}
		lines = append(lines, labelStyle.Render("  • Run 'tnr ports list' to see all forwarded ports"))

		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return "\n" + boxStyle.Render(content) + "\n\n"
	}

	return fmt.Sprintf("\n   %s %s\n\n", m.spinner.View(), m.message)
}
